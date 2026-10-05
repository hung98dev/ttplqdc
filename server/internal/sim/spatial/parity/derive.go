package parity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"thinhthan/internal/sim/spatial/geometry"
)

// Derive mirrors the C# GeometryExporter pipeline (contract §7): it reads a
// collision-only scene's YAML, applies the §2.5 binary32-rational
// quantization once per coordinate, classifies segments by the collider Game
// Object's `seg.<kind>` name against the §7.3 slope rules, sorts every array
// by id and writes the canonical schema-v1 bytes. The committed
// maps/<space_id>.geom.json must be byte-identical to this output.
//
// Scene conventions (authored per IMP-062):
//   - one GameObject `meta` carrying a GeometryMeta component: spaceId,
//     spaceKind, layoutProfile, boundsMaxX, boundsMaxY (meters)
//   - EdgeCollider2D on `ServerGeometry`-tagged GameObjects named seg.<kind>[.n]
//     under the `segments` root (points are local; transform position is the
//     world→map offset added in the rational domain)
//   - empty GameObjects named by anchor id under the `anchors` root
//   - GameObjects under the `camera_regions` root carrying a BoxCollider2D
//     whose offset/size is the region rectangle
//
// content_revision is the semantic revision of the exported content itself:
// lowercase SHA-256 hex of the canonical document with the field left empty.
const (
	schemaVersion = 1

	segPrefix      = "seg."
	anchorsRoot    = "anchors"
	regionsRoot    = "camera_regions"
	metaObjectName = "meta"
)

// SceneModel is the parsed collision scene in float meters.
type SceneModel struct {
	SpaceID       string
	SpaceKind     string
	LayoutProfile string
	BoundsMaxX    float64
	BoundsMaxY    float64
	Segments      []SceneSegment
	Regions       []SceneRegion
	Anchors       []SceneAnchor
}

type SceneSegment struct {
	KindToken string // name token after "seg."
	X1, Y1    float64
	X2, Y2    float64
}

type SceneRegion struct {
	MinX, MinY, MaxX, MaxY float64
}

type SceneAnchor struct {
	ID   string
	X, Y float64
}

// ---- Unity scene YAML subset parser ----------------------------------------

var (
	docHeaderRe  = regexp.MustCompile(`^--- !u!(\d+) &(\d+)`)
	gameObjectRe = regexp.MustCompile(`^GameObject:`)
	mNameRe      = regexp.MustCompile(`^  m_Name: (.*)$`)
	mTagRe       = regexp.MustCompile(`^  m_TagString: (.*)$`)
	mGObjRe      = regexp.MustCompile(`^  m_GameObject: \{fileID: (\d+)\}`)
	mFatherRe    = regexp.MustCompile(`^  m_Father: \{fileID: (\d+)\}`)
	mPosRe       = regexp.MustCompile(`^  m_LocalPosition: \{x: ([^,]+), y: ([^,]+), z: [^}]+\}`)
	mOffsetRe    = regexp.MustCompile(`^  m_Offset: \{x: ([^,]+), y: ([^}]+)\}`)
	mSizeRe      = regexp.MustCompile(`^  m_Size: \{x: ([^,]+), y: ([^}]+)\}`)
	mPointsRe    = regexp.MustCompile(`^  m_Points:`)
	pointItemRe  = regexp.MustCompile(`^  - \{x: ([^,]+), y: ([^}]+)\}`)
	mScriptRe    = regexp.MustCompile(`^  m_Script: \{fileID: \d+, guid: ([0-9a-f]+), type: \d+\}`)
	spaceIDRe    = regexp.MustCompile(`^  spaceId: (.*)$`)
	spaceKindRe  = regexp.MustCompile(`^  spaceKind: (.*)$`)
	layoutRe     = regexp.MustCompile(`^  layoutProfile: (.*)$`)
	boundsXRe    = regexp.MustCompile(`^  boundsMaxX: (.*)$`)
	boundsYRe    = regexp.MustCompile(`^  boundsMaxY: (.*)$`)
)

type yamlDoc struct {
	classID int64
	fileID  int64
	lines   []string
}

// ParseScene decodes the subset of Unity scene YAML the exporter consumes.
func ParseScene(data []byte) (*SceneModel, error) {
	var docs []yamlDoc
	for _, line := range strings.Split(string(data), "\n") {
		if m := docHeaderRe.FindStringSubmatch(line); m != nil {
			cid, _ := strconv.ParseInt(m[1], 10, 64)
			fid, _ := strconv.ParseInt(m[2], 10, 64)
			docs = append(docs, yamlDoc{classID: cid, fileID: fid})
			continue
		}
		if len(docs) > 0 {
			docs[len(docs)-1].lines = append(docs[len(docs)-1].lines, line)
		}
	}

	type gob struct {
		name string
		tag  string
	}
	type xform struct {
		goid   int64
		father int64
		x, y   float64
	}
	type edgeC struct {
		goid    int64
		offsetX float64
		offsetY float64
		points  [][2]float64
	}
	type boxC struct {
		goid    int64
		offsetX float64
		offsetY float64
		sizeX   float64
		sizeY   float64
	}
	type monoF struct {
		goid   int64
		fields map[string]string
	}

	gos := map[int64]gob{}
	tfs := map[int64]xform{} // transform fileID -> record
	tfByGO := map[int64]int64{}
	var edges []edgeC
	var boxes []boxC
	var metas []monoF

	monoFields := map[string]*regexp.Regexp{
		"spaceId": spaceIDRe, "spaceKind": spaceKindRe, "layoutProfile": layoutRe,
		"boundsMaxX": boundsXRe, "boundsMaxY": boundsYRe,
	}

	for _, d := range docs {
		switch d.classID {
		case 1: // GameObject
			var g gob
			for _, ln := range d.lines {
				if m := mNameRe.FindStringSubmatch(ln); m != nil {
					g.name = strings.TrimSpace(m[1])
				}
				if m := mTagRe.FindStringSubmatch(ln); m != nil {
					g.tag = strings.TrimSpace(m[1])
				}
			}
			gos[d.fileID] = g
		case 4: // Transform
			var t xform
			for _, ln := range d.lines {
				if m := mGObjRe.FindStringSubmatch(ln); m != nil {
					t.goid, _ = strconv.ParseInt(m[1], 10, 64)
				}
				if m := mFatherRe.FindStringSubmatch(ln); m != nil {
					t.father, _ = strconv.ParseInt(m[1], 10, 64)
				}
				if m := mPosRe.FindStringSubmatch(ln); m != nil {
					t.x = parseF(m[1])
					t.y = parseF(m[2])
				}
			}
			tfs[d.fileID] = t
			tfByGO[t.goid] = d.fileID
		case 68: // EdgeCollider2D
			var e edgeC
			inPoints := false
			for _, ln := range d.lines {
				if m := mGObjRe.FindStringSubmatch(ln); m != nil {
					e.goid, _ = strconv.ParseInt(m[1], 10, 64)
				}
				if m := mOffsetRe.FindStringSubmatch(ln); m != nil {
					e.offsetX = parseF(m[1])
					e.offsetY = parseF(m[2])
				}
				if mPointsRe.MatchString(ln) {
					inPoints = true
					continue
				}
				if inPoints {
					if m := pointItemRe.FindStringSubmatch(ln); m != nil {
						e.points = append(e.points, [2]float64{parseF(m[1]), parseF(m[2])})
						continue
					}
					inPoints = false
				}
			}
			edges = append(edges, e)
		case 61: // BoxCollider2D
			var b boxC
			for _, ln := range d.lines {
				if m := mGObjRe.FindStringSubmatch(ln); m != nil {
					b.goid, _ = strconv.ParseInt(m[1], 10, 64)
				}
				if m := mOffsetRe.FindStringSubmatch(ln); m != nil {
					b.offsetX = parseF(m[1])
					b.offsetY = parseF(m[2])
				}
				if m := mSizeRe.FindStringSubmatch(ln); m != nil {
					b.sizeX = parseF(m[1])
					b.sizeY = parseF(m[2])
				}
			}
			boxes = append(boxes, b)
		case 114: // MonoBehaviour
			var mf monoF
			mf.fields = map[string]string{}
			for _, ln := range d.lines {
				if m := mGObjRe.FindStringSubmatch(ln); m != nil {
					mf.goid, _ = strconv.ParseInt(m[1], 10, 64)
				}
				for k, re := range monoFields {
					if m := re.FindStringSubmatch(ln); m != nil {
						mf.fields[k] = strings.TrimSpace(m[1])
					}
				}
			}
			metas = append(metas, mf)
		}
	}

	// Transform hierarchy helpers: world position = sum of local positions up
	// the father chain (authored scenes use no rotation/scale on the geometry
	// roots — the exporter fails on rotated/scaled geometry colliders anyway).
	worldPos := func(goid int64) (float64, float64, bool) {
		tfid, ok := tfByGO[goid]
		if !ok {
			return 0, 0, false
		}
		x, y := 0.0, 0.0
		for depth := 0; depth < 64 && tfid != 0; depth++ {
			t := tfs[tfid]
			x += t.x
			y += t.y
			tfid = t.father
		}
		return x, y, true
	}
	under := func(goid int64, rootName string) bool {
		// true when the GameObject's transform ANCESTOR chain reaches a root
		// named rootName (the object itself does not count).
		tfid, ok := tfByGO[goid]
		if !ok {
			return false
		}
		tfid = tfs[tfid].father
		for depth := 0; depth < 64 && tfid != 0; depth++ {
			t := tfs[tfid]
			if g, ok := gos[t.goid]; ok && g.name == rootName {
				return true
			}
			tfid = t.father
		}
		return false
	}

	m := &SceneModel{}
	seenMeta := false
	for _, mf := range metas {
		g, ok := gos[mf.goid]
		if !ok || g.name != metaObjectName {
			continue
		}
		if seenMeta {
			return nil, fmt.Errorf("parity: scene has duplicate %q objects", metaObjectName)
		}
		seenMeta = true
		m.SpaceID = mf.fields["spaceId"]
		m.SpaceKind = mf.fields["spaceKind"]
		m.LayoutProfile = mf.fields["layoutProfile"]
		m.BoundsMaxX = parseF(mf.fields["boundsMaxX"])
		m.BoundsMaxY = parseF(mf.fields["boundsMaxY"])
	}
	if !seenMeta {
		return nil, fmt.Errorf("parity: scene has no %q meta object", metaObjectName)
	}
	if m.SpaceID == "" || m.SpaceKind == "" || m.LayoutProfile == "" {
		return nil, fmt.Errorf("parity: meta object missing spaceId/spaceKind/layoutProfile")
	}

	for _, e := range edges {
		g, ok := gos[e.goid]
		if !ok || g.tag != "ServerGeometry" {
			continue
		}
		if !under(e.goid, "segments") {
			return nil, fmt.Errorf("parity: ServerGeometry collider %q not under segments root", g.name)
		}
		if len(e.points) != 2 {
			return nil, fmt.Errorf("parity: ServerGeometry collider %q has %d points, want 2", g.name, len(e.points))
		}
		if !strings.HasPrefix(g.name, segPrefix) {
			return nil, fmt.Errorf("parity: ServerGeometry collider %q missing %q name prefix", g.name, segPrefix)
		}
		wx, wy, _ := worldPos(e.goid)
		m.Segments = append(m.Segments, SceneSegment{
			KindToken: strings.TrimPrefix(g.name, segPrefix),
			X1:        wx + e.offsetX + e.points[0][0],
			Y1:        wy + e.offsetY + e.points[0][1],
			X2:        wx + e.offsetX + e.points[1][0],
			Y2:        wy + e.offsetY + e.points[1][1],
		})
	}

	for _, b := range boxes {
		if _, ok := gos[b.goid]; !ok || !under(b.goid, regionsRoot) {
			continue
		}
		wx, wy, _ := worldPos(b.goid)
		cx, cy := wx+b.offsetX, wy+b.offsetY
		m.Regions = append(m.Regions, SceneRegion{
			MinX: cx - b.sizeX/2, MinY: cy - b.sizeY/2,
			MaxX: cx + b.sizeX/2, MaxY: cy + b.sizeY/2,
		})
	}

	for id, g := range gos {
		if !under(id, anchorsRoot) {
			continue
		}
		wx, wy, _ := worldPos(id)
		m.Anchors = append(m.Anchors, SceneAnchor{ID: g.name, X: wx, Y: wy})
	}
	return m, nil
}

func parseF(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	if err != nil {
		return math.NaN()
	}
	return float64(f)
}

// ---- quantization (contract §2.5) -------------------------------------------

// mmFloat quantizes a world-meters coordinate through the exact binary32
// rational domain: decode sign/mantissa/exponent, scale by 1000, RoundDiv
// once. NaN/Inf/overflow are rejected (§2.5).
func mmFloat(v float64) (int64, error) {
	f := float32(v)
	if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
		return 0, fmt.Errorf("non-finite coordinate %v", v)
	}
	if math.Abs(float64(f)) > 9.0e15 {
		// |v|*1000 would overflow int64 mm.
		return 0, fmt.Errorf("coordinate %v overflows mm range", v)
	}
	num, exp2 := binary32Rational(f)
	if exp2 >= 0 {
		// value = num << exp2 fits int64 (guarded |f| ≤ 9e15); ×1000 then fits.
		return (num << exp2) * 1000, nil
	}
	return roundDivPow2(num*1000, uint(-exp2)), nil
}

// binary32Rational decomposes an IEEE-754 binary32 into n × 2^exp2 exactly.
func binary32Rational(f float32) (num int64, exp2 int64) {
	bits := math.Float32bits(f)
	sign := int64(1)
	if bits>>31 != 0 {
		sign = -1
	}
	exp := int64((bits >> 23) & 0xff)
	mant := int64(bits & 0x7fffff)
	if exp == 0 {
		// subnormal: mant × 2^-149
		return sign * mant, -149
	}
	// normal: (2^23 + mant) × 2^(exp-150)
	return sign * (mant + 1<<23), exp - 150
}

// roundDivPow2 divides n by 2^k, halves away from zero (contract §2.2).
// Equivalent to geometry.RoundDiv(n, 1<<k) without materializing 2^k.
func roundDivPow2(n int64, k uint) int64 {
	neg := n < 0
	a := n
	if neg {
		a = -a
	}
	var q int64
	switch {
	case k == 0:
		q = a
	case k >= 64:
		// a < 2^63 < 2^k; |remainder| < 2^(k-1) → rounds to 0.
		q = 0
	default:
		q = a >> k
		if r := a - (q << k); r >= int64(1)<<(k-1) {
			q++
		}
	}
	if neg {
		return -q
	}
	return q
}

// ---- canonical writer -------------------------------------------------------

type qSegment struct {
	id     int64
	kind   geometry.SegmentKind
	x1, y1 int64
	x2, y2 int64
}

type qRegion struct {
	id                     int64
	minX, minY, maxX, maxY int64
}

type qAnchor struct {
	id   string
	x, y int64
}

// CanonicalJSON quantizes the scene model and emits the schema-v1 document in
// the contract's key/array order, byte-identical to the C# exporter output.
func CanonicalJSON(m *SceneModel) ([]byte, error) {
	qsegs := make([]qSegment, 0, len(m.Segments))
	for i, s := range m.Segments {
		kind, ok := kindForToken(s.KindToken)
		if !ok {
			return nil, fmt.Errorf("parity: segment %d unknown kind token %q", i, s.KindToken)
		}
		x1, err := mmFloat(s.X1)
		if err != nil {
			return nil, fmt.Errorf("parity: segment %d x1: %w", i, err)
		}
		y1, err := mmFloat(s.Y1)
		if err != nil {
			return nil, fmt.Errorf("parity: segment %d y1: %w", i, err)
		}
		x2, err := mmFloat(s.X2)
		if err != nil {
			return nil, fmt.Errorf("parity: segment %d x2: %w", i, err)
		}
		y2, err := mmFloat(s.Y2)
		if err != nil {
			return nil, fmt.Errorf("parity: segment %d y2: %w", i, err)
		}
		// Normalize direction: x1<x2 for non-vertical; vertical wall y1<y2.
		if x1 > x2 || (x1 == x2 && y1 > y2) {
			x1, x2 = x2, x1
			y1, y2 = y2, y1
		}
		if err := checkKindSlope(kind, x1, y1, x2, y2); err != nil {
			return nil, fmt.Errorf("parity: segment %d (%s): %w", i, s.KindToken, err)
		}
		qsegs = append(qsegs, qSegment{kind: kind, x1: x1, y1: y1, x2: x2, y2: y2})
	}
	sort.Slice(qsegs, func(i, j int) bool {
		a, b := qsegs[i], qsegs[j]
		if a.x1 != b.x1 {
			return a.x1 < b.x1
		}
		if a.y1 != b.y1 {
			return a.y1 < b.y1
		}
		if a.x2 != b.x2 {
			return a.x2 < b.x2
		}
		return a.y2 < b.y2
	})
	for i := range qsegs {
		qsegs[i].id = int64(i + 1)
	}

	qregs := make([]qRegion, 0, len(m.Regions))
	for i, r := range m.Regions {
		minX, err := mmFloat(r.MinX)
		if err != nil {
			return nil, fmt.Errorf("parity: region %d min_x: %w", i, err)
		}
		minY, err := mmFloat(r.MinY)
		if err != nil {
			return nil, fmt.Errorf("parity: region %d min_y: %w", i, err)
		}
		maxX, err := mmFloat(r.MaxX)
		if err != nil {
			return nil, fmt.Errorf("parity: region %d max_x: %w", i, err)
		}
		maxY, err := mmFloat(r.MaxY)
		if err != nil {
			return nil, fmt.Errorf("parity: region %d max_y: %w", i, err)
		}
		qregs = append(qregs, qRegion{minX: minX, minY: minY, maxX: maxX, maxY: maxY})
	}
	sort.Slice(qregs, func(i, j int) bool {
		a, b := qregs[i], qregs[j]
		if a.minX != b.minX {
			return a.minX < b.minX
		}
		return a.minY < b.minY
	})
	for i := range qregs {
		qregs[i].id = int64(i + 1)
	}

	qanch := make([]qAnchor, 0, len(m.Anchors))
	for i, a := range m.Anchors {
		if a.ID == "" {
			return nil, fmt.Errorf("parity: anchor %d has empty id", i)
		}
		x, err := mmFloat(a.X)
		if err != nil {
			return nil, fmt.Errorf("parity: anchor %s x: %w", a.ID, err)
		}
		y, err := mmFloat(a.Y)
		if err != nil {
			return nil, fmt.Errorf("parity: anchor %s y: %w", a.ID, err)
		}
		qanch = append(qanch, qAnchor{id: a.ID, x: x, y: y})
	}
	sort.Slice(qanch, func(i, j int) bool { return qanch[i].id < qanch[j].id })

	maxX, err := mmFloat(m.BoundsMaxX)
	if err != nil {
		return nil, fmt.Errorf("parity: boundsMaxX: %w", err)
	}
	maxY, err := mmFloat(m.BoundsMaxY)
	if err != nil {
		return nil, fmt.Errorf("parity: boundsMaxY: %w", err)
	}

	// content_revision = semantic revision of this exported content: SHA-256
	// of the canonical document with the field empty (content_authoring
	// contract §5 format — 64 lowercase hex).
	rev := contentRevision(m.SpaceID, m.SpaceKind, m.LayoutProfile, maxX, maxY, qsegs, qregs, qanch)
	return renderJSON(m.SpaceID, m.SpaceKind, m.LayoutProfile, rev, maxX, maxY, qsegs, qregs, qanch), nil
}

func kindForToken(tok string) (geometry.SegmentKind, bool) {
	base := tok
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "ground":
		return geometry.SolidGround, true
	case "slope":
		return geometry.Slope, true
	case "wall":
		return geometry.Wall, true
	case "ceiling":
		return geometry.Ceiling, true
	case "oneway":
		return geometry.OneWayPlatform, true
	}
	return 0, false
}

// checkKindSlope enforces the §7.3 kind slope rules on quantized mm coords,
// identical to geometry.checkSlopeRule.
func checkKindSlope(k geometry.SegmentKind, x1, y1, x2, y2 int64) error {
	dx, dy := x2-x1, y2-y1
	adx, ady := dx, dy
	if adx < 0 {
		adx = -adx
	}
	if ady < 0 {
		ady = -ady
	}
	le5 := adx > 0 && ady*1000000 <= adx*87489
	le45 := ady <= adx
	if x1 == x2 {
		if k != geometry.Wall || y1 >= y2 {
			return fmt.Errorf("vertical segment must be WALL with y1 < y2")
		}
		return nil
	}
	if x1 >= x2 {
		return fmt.Errorf("non-vertical segment requires x1 < x2")
	}
	switch k {
	case geometry.SolidGround, geometry.OneWayPlatform:
		if !le5 {
			return fmt.Errorf("kind %s requires |slope| <= 5 degrees", k)
		}
	case geometry.Slope:
		if le5 || !le45 {
			return fmt.Errorf("kind SLOPE requires 5 < |slope| <= 45 degrees")
		}
	case geometry.Wall:
		if le45 {
			return fmt.Errorf("kind WALL requires |slope| > 45 degrees")
		}
	case geometry.Ceiling:
		if !le45 {
			return fmt.Errorf("kind CEILING requires |slope| <= 45 degrees")
		}
	}
	return nil
}

func qJSON(s string) string {
	// anchor ids and space ids are [a-z0-9._-]; escape defensively anyway.
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(c)
		default:
			if c < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func renderJSON(spaceID, kind, profile, rev string, maxX, maxY int64, segs []qSegment, regs []qRegion, anchs []qAnchor) []byte {
	var b strings.Builder
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"schema_version\": %d,\n", schemaVersion)
	fmt.Fprintf(&b, "  \"space_id\": %s,\n", qJSON(spaceID))
	fmt.Fprintf(&b, "  \"space_kind\": %s,\n", qJSON(kind))
	fmt.Fprintf(&b, "  \"layout_profile\": %s,\n", qJSON(profile))
	fmt.Fprintf(&b, "  \"content_revision\": %s,\n", qJSON(rev))
	fmt.Fprintf(&b, "  \"bounds_mm\": { \"max_x\": %d, \"max_y\": %d },\n", maxX, maxY)
	b.WriteString("  \"segments\": [\n")
	for i, s := range segs {
		comma := ","
		if i == len(segs)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "    { \"id\": %d, \"kind\": %s, \"x1\": %d, \"y1\": %d, \"x2\": %d, \"y2\": %d }%s\n",
			s.id, qJSON(s.kind.String()), s.x1, s.y1, s.x2, s.y2, comma)
	}
	b.WriteString("  ],\n")
	b.WriteString("  \"camera_regions\": [\n")
	for i, r := range regs {
		comma := ","
		if i == len(regs)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "    { \"id\": %d, \"min_x\": %d, \"min_y\": %d, \"max_x\": %d, \"max_y\": %d }%s\n",
			r.id, r.minX, r.minY, r.maxX, r.maxY, comma)
	}
	b.WriteString("  ],\n")
	b.WriteString("  \"anchors\": [\n")
	for i, a := range anchs {
		comma := ","
		if i == len(anchs)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "    { \"id\": %s, \"x\": %d, \"y\": %d }%s\n", qJSON(a.id), a.x, a.y, comma)
	}
	b.WriteString("  ]\n")
	b.WriteString("}\n")
	return []byte(b.String())
}

func contentRevision(spaceID, kind, profile string, maxX, maxY int64, segs []qSegment, regs []qRegion, anchs []qAnchor) string {
	doc := renderJSON(spaceID, kind, profile, "", maxX, maxY, segs, regs, anchs)
	sum := sha256.Sum256(doc)
	return hex.EncodeToString(sum[:])
}

// ---- topology helpers (test-side) -------------------------------------------

// DeadEndAnchors returns leaf walkable endpoints whose unique branch path
// exceeds 0.5 reference screens (12.8 m) and must terminate in a content
// anchor (§6.1 main-route rule).
func DeadEndViolations(g *geometry.Geometry, anchors map[string][2]int64) []string {
	// Build node graph over walkable segments: nodes are quantized endpoints;
	// an endpoint lying strictly inside another walkable segment is a
	// T-junction merged onto that segment.
	type node struct{ x, y int64 }
	var segs []geometry.Segment
	for _, s := range g.Segments {
		if s.Kind.IsWalkable() && s.X1 != s.X2 {
			segs = append(segs, s)
		}
	}
	deg := map[node]int{}
	adj := map[node][]node{}
	for _, s := range segs {
		a, b := node{s.X1, s.Y1}, node{s.X2, s.Y2}
		deg[a]++
		deg[b]++
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	// T-junctions: endpoint strictly inside another walkable segment raises
	// that point to a junction.
	junction := map[node]bool{}
	for n, d := range deg {
		if d >= 3 {
			junction[n] = true
		}
	}
	for _, s := range segs {
		for n := range deg {
			if (n.x == s.X1 && n.y == s.Y1) || (n.x == s.X2 && n.y == s.Y2) {
				continue
			}
			if n.x > s.X1 && n.x < s.X2 {
				y := geometry.SurfaceHeight(s, n.x)
				if y == n.y {
					junction[n] = true
				}
			}
		}
	}
	var violations []string
	for n, d := range deg {
		if d != 1 {
			continue
		}
		// walk the unique chain back to a junction or another leaf
		path := int64(0)
		cur := n
		prev := node{-1, -1}
		for {
			nexts := adj[cur]
			var nxt node
			found := false
			for _, c := range nexts {
				if c != prev {
					nxt, found = c, true
					break
				}
			}
			if !found {
				break
			}
			dx, dy := nxt.x-cur.x, nxt.y-cur.y
			path += int64(math.Sqrt(float64(dx*dx + dy*dy)))
			prev, cur = cur, nxt
			if junction[cur] || deg[cur] != 2 {
				break
			}
		}
		if path <= 12800 {
			continue
		}
		// require an anchor near the leaf tip (within 2m)
		ok := false
		for _, pos := range anchors {
			dx, dy := pos[0]-n.x, pos[1]-n.y
			if dx*dx+dy*dy <= 2000*2000 {
				ok = true
				break
			}
		}
		if !ok {
			violations = append(violations, fmt.Sprintf("dead-end at (%d,%d) length %dmm without terminal anchor", n.x, n.y, path))
		}
	}
	return violations
}
