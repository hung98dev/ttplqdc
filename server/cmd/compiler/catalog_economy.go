package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileEconomy — economy_catalog.md driver: faucet bands, cosmetic/bound/
// special sinks, bound offers, telemetry thresholds.
func compileEconomy(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		raw := b.Raw
		switch {
		case strings.Contains(raw, "Combat Faucet Bands"):
			econFaucetBands(c, f, b)
		case strings.Contains(raw, "Common Sinks"):
			econBulletParams(c, f, b, "Common Sinks", "common_sink")
		case strings.Contains(raw, "common / Cosmetic Sink") || strings.Contains(raw, "common` / Cosmetic Sink"):
			econCosmeticPrices(c, f, b, "currency.common` Cosmetic Sink Catalog", "common")
		case strings.Contains(raw, "bound / Sources"):
			econBoundSources(c, f, b)
		case strings.Contains(raw, "bound / Sinks"):
			econBoundSinks(c, f, b)
		case strings.Contains(raw, "One-Time PvE Sources"):
			econSpecialSources(c, f, b)
		case strings.Contains(raw, "Launch Cosmetic Sinks") || strings.Contains(raw, "Extended Special"):
			econSpecialSinks(c, f, b)
		case strings.Contains(raw, "Affordability Targets"):
			econBulletParams(c, f, b, "Affordability Targets", "affordability_target")
		case strings.Contains(raw, "Inflation / Deflation Telemetry"):
			econTelemetry(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"economy binding %q has no driver", raw)
		}
	}
}

var econRangeRe = regexp.MustCompile(`^([0-9,]+)\.\.([0-9,]+)`)
var econCostRe = regexp.MustCompile(`^(cosmetic\.[a-z0-9_.]+)\s+cost\s+([0-9,]+)\s+(common|special|bound)`)

func econFindSub(f *File, title string) *Section {
	var hit *Section
	var walk func(s *Section)
	walk = func(s *Section) {
		for _, ch := range s.Children {
			if strings.Contains(ch.Title, title) {
				hit = ch
				return
			}
			walk(ch)
		}
	}
	walk(f.Root)
	return hit
}

func econFaucetBands(c *Ctx, f *File, b *SourceBinding) {
	kinds := map[string]string{
		"NORMAL monster": "NORMAL", "ELITE monster": "ELITE", "Major boss": "MAJOR_BOSS",
	}
	for sub, kind := range kinds {
		sec := econFindSub(f, sub)
		if sec == nil {
			continue
		}
		for _, bl := range sec.Content {
			if bl.Kind != BlockTable {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				tier := cellAt(row, 0).Scalar()
				m := econRangeRe.FindStringSubmatch(cellAt(row, 1).Scalar())
				if m == nil {
					c.Diags.Addf(config.DiagTableSyntaxError, f.Path, row[0].Line,
						"bad currency range %q", cellAt(row, 1).Scalar())
					continue
				}
				lo, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(m[1], ",", ""))
				hi, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(m[2], ",", ""))
				c.Emit(f.Name, b.Raw, "faucet_band",
					[]config.Value{config.VStr(kind), config.VStr(tier)},
					map[string]config.Value{
						"monster_kind": config.VStr(kind),
						"tier":         config.VStr(tier),
						"lo":           lo,
						"hi":           hi,
						"min":          lo,
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func econBulletParams(c *Ctx, f *File, b *SourceBinding, secTitle, fam string) {
	sec := econFindSub(f, secTitle)
	if sec == nil {
		c.consumed(f, b)
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.Prose {
			l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "-"))
			l = strings.TrimSuffix(l, ",")
			l = strings.TrimSuffix(l, ".")
			if l == "" {
				continue
			}
			c.EmitParam(f.Name, b.Raw, fam,
				[]config.Value{config.VStr(l)},
				map[string]config.Value{"name": config.VStr(l)}, bl.Line+1+j)
		}
	}
	c.consumed(f, b)
}

func econCosmeticPrices(c *Ctx, f *File, b *SourceBinding, parentTitle, currency string) {
	parent := econFindSub(f, parentTitle)
	if parent == nil {
		parent = f.Root
	}
	for _, bl := range flattenBlocks(parent) {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := econCostRe.FindStringSubmatch(l); m != nil {
				if m[3] != currency {
					continue
				}
				cost, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(m[2], ",", ""))
				c.Emit(f.Name, b.Raw, "cosmetic_price",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"cosmetic_id": config.VStr(m[1]),
						"currency":    config.VStr("currency." + currency),
						"cost":        cost,
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func econBoundSources(c *Ctx, f *File, b *SourceBinding) {
	sec := econFindSub(f, "currency.bound`")
	if sec == nil {
		sec = f.Root
	}
	sources := econFindSub(f, "Sources")
	if sources == nil {
		sources = sec
	}
	for _, sub := range sources.Children {
		fam := ""
		switch {
		case strings.HasPrefix(sub.Title, "SIDE quest"):
			fam = "SIDE_QUEST"
		case strings.HasPrefix(sub.Title, "Dungeon completion"):
			fam = "DUNGEON_DAILY"
		case strings.HasPrefix(sub.Title, "Ranked PvP"):
			fam = "RANKED_PVP"
		case strings.HasPrefix(sub.Title, "Guild War"):
			fam = "GUILD_WAR"
		case strings.HasPrefix(sub.Title, "Daily MYSTERY"):
			fam = "DAILY_MYSTERY"
		}
		if fam == "" {
			continue
		}
		var tiers []config.Value
		var ruleText []config.Value
		for _, bl := range sub.Content {
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if m := regexp.MustCompile(`^(T[0-9]|ENDGAME_L60|T[0-9]:)\s+([0-9]+)`).FindStringSubmatch(l); m != nil {
					t := strings.TrimSuffix(m[1], ":")
					v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
					tiers = append(tiers, config.VRec(map[string]config.Value{
						"tier": config.VStr(t), "amount": v,
					}))
					continue
				}
				if l != "" {
					ruleText = append(ruleText, config.VStr(l))
				}
			}
			for _, l := range bl.Prose {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "bound =") || strings.HasPrefix(l, "20 bound") || strings.HasPrefix(l, "50 bound") {
					ruleText = append(ruleText, config.VStr(l))
				}
			}
		}
		fields := map[string]config.Value{
			"source_family": config.VStr(fam),
			"currency":      config.VStr("currency.bound"),
		}
		if len(tiers) > 0 {
			fields["tier_amounts"] = config.VList(tiers...)
		}
		if len(ruleText) > 0 {
			fields["rule"] = config.VSet(ruleText...)
		}
		c.Emit(f.Name, b.Raw, "bound_faucet",
			[]config.Value{config.VStr(fam)}, fields, sub.Line)
	}
	c.consumed(f, b)
}

var offerRe = regexp.MustCompile(`^(offer\.bound\.[a-z0-9_.]+)\s*->\s*(item\.[a-z0-9_.]+)\s+cost\s+([0-9]+)\s+bound`)

func econBoundSinks(c *Ctx, f *File, b *SourceBinding) {
	sinks := econFindSub(f, "Sinks")
	if sinks == nil {
		sinks = f.Root
	}
	for _, bl := range flattenBlocks(sinks) {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := offerRe.FindStringSubmatch(l); m != nil {
				cost, _ := (TypeSpec{Name: "int"}).ParseValue(m[3])
				c.Emit(f.Name, b.Raw, "bound_offer",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"offer_id": config.VStr(m[1]),
						"item_id":  config.VStr(m[2]),
						"cost":     cost,
						"currency": config.VStr("currency.bound"),
						"binding":  config.VStr("CHARACTER_BOUND"),
						"trigger":  config.VStr("ON_ACQUIRE"),
					}, bl.Line+1+j)
			}
		}
	}
	// Bound-Purchase Output Rule
	rule := econFindSub(f, "Bound-Purchase Output Rule")
	if rule != nil {
		for _, bl := range rule.Content {
			var fields []config.Value
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if l != "" {
					fields = append(fields, config.VStr(l))
				}
			}
			if len(fields) > 0 {
				c.EmitParam(f.Name, b.Raw, "bound_purchase_rule",
					[]config.Value{config.VStr("output")},
					map[string]config.Value{"fields": config.VSet(fields...)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}

func econSpecialSources(c *Ctx, f *File, b *SourceBinding) {
	sec := econFindSub(f, "One-Time PvE Sources")
	if sec == nil {
		return
	}
	srcRe := regexp.MustCompile(`^(.+?)\s*->\s*([0-9]+)\s+special`)
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := srcRe.FindStringSubmatch(l); m != nil {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				c.Emit(f.Name, b.Raw, "special_faucet",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"source_key": config.VStr(m[1]),
						"amount":     v,
						"currency":   config.VStr("currency.special"),
						"scope":      config.VStr("CHARACTER"),
					}, bl.Line+1+j)
				continue
			}
			if strings.HasPrefix(l, "economy.special.") {
				c.EmitParam(f.Name, b.Raw, "special_key_namespace",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"namespace": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func econSpecialSinks(c *Ctx, f *File, b *SourceBinding) {
	// base `20 special -> cosmetic.id` + fabric alternatives + extended cost fences
	sinkRe := regexp.MustCompile(`^([0-9]+)\s+special\s*->\s*(cosmetic\.[a-z0-9_.]+)`)
	fabRe := regexp.MustCompile(`^([0-9]+)\s+fabric\s*->\s*(cosmetic\.[a-z0-9_.]+)`)
	root := f.Root
	for _, bl := range flattenBlocks(root) {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := sinkRe.FindStringSubmatch(l); m != nil {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				c.Emit(f.Name, b.Raw, "special_sink",
					[]config.Value{config.VStr(m[2])},
					map[string]config.Value{
						"cosmetic_id": config.VStr(m[2]),
						"cost":        v,
						"currency":    config.VStr("currency.special"),
					}, bl.Line+1+j)
				continue
			}
			if m := fabRe.FindStringSubmatch(l); m != nil {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[1])
				c.Emit(f.Name, b.Raw, "material_sink",
					[]config.Value{config.VStr(m[2])},
					map[string]config.Value{
						"cosmetic_id": config.VStr(m[2]),
						"cost":        v,
						"material":    config.VStr("item.material.vai_hoa_van"),
					}, bl.Line+1+j)
				continue
			}
			if m := econCostRe.FindStringSubmatch(l); m != nil && m[3] == "special" {
				cost, _ := (TypeSpec{Name: "int"}).ParseValue(strings.ReplaceAll(m[2], ",", ""))
				c.Emit(f.Name, b.Raw, "cosmetic_price",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"cosmetic_id": config.VStr(m[1]),
						"currency":    config.VStr("currency.special"),
						"cost":        cost,
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func econTelemetry(c *Ctx, f *File, b *SourceBinding) {
	sec := econFindSub(f, "Inflation / Deflation Telemetry")
	if sec == nil {
		c.consumed(f, b)
		return
	}
	for _, bl := range flattenBlocks(sec) {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			// metric = value / threshold lines
			kv := chestFieldRe.FindStringSubmatch(l)
			if kv != nil {
				c.EmitParam(f.Name, b.Raw, "telemetry_threshold",
					[]config.Value{config.VStr(kv[1])},
					map[string]config.Value{
						"metric": config.VStr(kv[1]),
						"value":  config.VStr(strings.TrimSpace(kv[2])),
					}, bl.Line+1+j)
			} else {
				c.EmitParam(f.Name, b.Raw, "telemetry_metric",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"metric": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}
