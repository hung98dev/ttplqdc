package geometry

import (
	"bytes"
	"encoding/json"
	"fmt"

	"thinhthan/internal/config"
)

// Parse strictly decodes a geom JSON document (schema_version 1) and validates
// it against the registered SpaceRecord for the space. It never panics and
// returns no partial result: any failure is a path-tagged *Error wrapping
// ErrMalformed (bad JSON, wrong types, non-integer numbers, unknown keys) or
// ErrSchema (contract violations found by Validate).
func Parse(data []byte, rec config.SpaceRecord) (*Geometry, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, malformed("$", err.Error())
	}
	if dec.More() {
		return nil, malformed("$", "trailing data after document")
	}

	top := objDecoder{raw: raw}
	if err := top.unknowns("schema_version", "space_id", "space_kind", "layout_profile",
		"content_revision", "bounds_mm", "segments", "camera_regions", "anchors"); err != nil {
		return nil, err
	}

	var (
		schemaVersion   int64
		spaceID         string
		spaceKindStr    string
		layoutProfile   string
		contentRevision string
		boundsRaw       map[string]json.RawMessage
		segmentsRaw     []json.RawMessage
		regionsRaw      []json.RawMessage
		anchorsRaw      []json.RawMessage
	)
	for _, f := range []struct {
		path string
		dst  any
	}{
		{"schema_version", &schemaVersion},
		{"space_id", &spaceID},
		{"space_kind", &spaceKindStr},
		{"layout_profile", &layoutProfile},
		{"content_revision", &contentRevision},
		{"bounds_mm", &boundsRaw},
		{"segments", &segmentsRaw},
		{"camera_regions", &regionsRaw},
		{"anchors", &anchorsRaw},
	} {
		if err := top.field(f.path, f.dst); err != nil {
			return nil, err
		}
	}

	bounds := objDecoder{raw: boundsRaw, base: "bounds_mm"}
	if err := bounds.unknowns("max_x", "max_y"); err != nil {
		return nil, err
	}
	var maxX, maxY int64
	if err := bounds.field("max_x", &maxX); err != nil {
		return nil, err
	}
	if err := bounds.field("max_y", &maxY); err != nil {
		return nil, err
	}

	kind, ok := ParseSpaceKind(spaceKindStr)
	if !ok {
		return nil, malformedPath("space_kind", fmt.Sprintf("unknown space_kind %q", spaceKindStr))
	}

	g := &Geometry{
		SpaceID:         spaceID,
		Kind:            kind,
		LayoutProfile:   layoutProfile,
		ContentRevision: contentRevision,
		schemaVersion:   schemaVersion,
	}
	g.BoundsMM.MaxX = maxX
	g.BoundsMM.MaxY = maxY

	segs := make([]Segment, 0, len(segmentsRaw))
	for i, sRaw := range segmentsRaw {
		path := fmt.Sprintf("segments[%d]", i)
		var sm map[string]json.RawMessage
		if err := unmarshalAt(path, sRaw, &sm); err != nil {
			return nil, err
		}
		sd := objDecoder{raw: sm, base: path}
		if err := sd.unknowns("id", "kind", "x1", "y1", "x2", "y2"); err != nil {
			return nil, err
		}
		var seg Segment
		var kindStr string
		for _, f := range []struct {
			key string
			dst any
		}{
			{"id", &seg.ID},
			{"kind", &kindStr},
			{"x1", &seg.X1},
			{"y1", &seg.Y1},
			{"x2", &seg.X2},
			{"y2", &seg.Y2},
		} {
			if err := sd.field(f.key, f.dst); err != nil {
				return nil, err
			}
		}
		sk, ok := ParseSegmentKind(kindStr)
		if !ok {
			return nil, malformedPath(path+".kind", fmt.Sprintf("unknown segment kind %q", kindStr))
		}
		seg.Kind = sk
		segs = append(segs, seg)
	}
	g.Segments = segs

	regions := make([]CameraRegion, 0, len(regionsRaw))
	for i, rRaw := range regionsRaw {
		path := fmt.Sprintf("camera_regions[%d]", i)
		var rm map[string]json.RawMessage
		if err := unmarshalAt(path, rRaw, &rm); err != nil {
			return nil, err
		}
		rd := objDecoder{raw: rm, base: path}
		if err := rd.unknowns("id", "min_x", "min_y", "max_x", "max_y"); err != nil {
			return nil, err
		}
		var reg CameraRegion
		for _, f := range []struct {
			key string
			dst any
		}{
			{"id", &reg.ID},
			{"min_x", &reg.MinX},
			{"min_y", &reg.MinY},
			{"max_x", &reg.MaxX},
			{"max_y", &reg.MaxY},
		} {
			if err := rd.field(f.key, f.dst); err != nil {
				return nil, err
			}
		}
		regions = append(regions, reg)
	}
	g.CameraRegions = regions

	anchors := make([]Anchor, 0, len(anchorsRaw))
	for i, aRaw := range anchorsRaw {
		path := fmt.Sprintf("anchors[%d]", i)
		var am map[string]json.RawMessage
		if err := unmarshalAt(path, aRaw, &am); err != nil {
			return nil, err
		}
		ad := objDecoder{raw: am, base: path}
		if err := ad.unknowns("id", "x", "y"); err != nil {
			return nil, err
		}
		var a Anchor
		if err := ad.field("id", &a.ID); err != nil {
			return nil, err
		}
		if err := ad.field("x", &a.X); err != nil {
			return nil, err
		}
		if err := ad.field("y", &a.Y); err != nil {
			return nil, err
		}
		anchors = append(anchors, a)
	}
	g.Anchors = anchors

	if err := Validate(g, &rec); err != nil {
		return nil, err
	}
	return g, nil
}

// schemaVersion is validated inside Parse (before the rest of Validate) so it
// lives on the struct privately.
func malformed(path, msg string) error {
	return &Error{Path: path, Msg: msg, sentinel: ErrMalformed}
}

func malformedPath(path, msg string) error {
	return malformed(path, msg)
}

// unmarshalAt decodes one RawMessage with UseNumber semantics: non-integer
// literals into int64 destinations fail here, giving path-tagged malformed
// errors instead of silent float coercion.
func unmarshalAt(path string, raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return malformed(path, err.Error())
	}
	if dec.More() {
		return malformed(path, "trailing data")
	}
	return nil
}

// objDecoder decodes one JSON object level with unknown-key rejection and
// path-tagged field errors.
type objDecoder struct {
	raw  map[string]json.RawMessage
	base string // "" for the document root
}

func (d objDecoder) path(key string) string {
	if d.base == "" {
		return key
	}
	return d.base + "." + key
}

func (d objDecoder) unknowns(keys ...string) error {
	allow := make(map[string]bool, len(keys))
	for _, k := range keys {
		allow[k] = true
	}
	for k := range d.raw {
		if !allow[k] {
			return malformed(d.path(k), "unknown key")
		}
	}
	return nil
}

func (d objDecoder) field(key string, dst any) error {
	raw, ok := d.raw[d.pathKey(key)]
	if !ok {
		return malformed(d.path(key), "missing required key")
	}
	return unmarshalAt(d.path(key), raw, dst)
}

func (d objDecoder) pathKey(key string) string { return key }
