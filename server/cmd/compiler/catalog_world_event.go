package main

import (
	"regexp"
	"strings"

	"thinhthan/internal/config"
)

// compileWorldEvent — world_event_catalog.md driver: spirit surge event,
// deterministic schedule derivation, variants, elite chain, participation,
// reward grants.
func compileWorldEvent(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		raw := b.Raw
		switch {
		case strings.Contains(raw, "text` fences") && strings.Contains(raw, "Event Identity"):
			weIdentity(c, f, b)
		case strings.Contains(raw, "H mod 5"):
			weAccessTable(c, f, b)
		case strings.Contains(raw, "Eligible Field Order"):
			weFieldOrder(c, f, b)
		case strings.Contains(raw, "Surge Variant"):
			weVariant(c, f, b)
		case strings.Contains(raw, "Temporary Spawn Groups"):
			weSpawnGroups(c, f, b)
		case strings.Contains(raw, "Elite Event Chain"):
			weEliteChain(c, f, b)
		case strings.Contains(raw, "Participation"):
			weParticipation(c, f, b)
		case strings.Contains(raw, "Rewards"):
			weRewards(c, f, b)
		case strings.Contains(raw, "Restart"):
			weRestart(c, f, b)
		case strings.Contains(raw, "Validation") || strings.Contains(raw, "Invariants"):
			weValidation(c, f, b)
		default:
			c.Diags.Addf(config.DiagUnregisteredSource, f.Path, b.Line,
				"world_event binding %q has no driver", raw)
		}
	}
}

func weFences(c *Ctx, f *File, b *SourceBinding, sec *Section, fam string) {
	for _, bl := range sec.Content {
		var lines []config.Value
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l != "" {
				lines = append(lines, config.VStr(l))
			}
		}
		if len(lines) > 0 {
			c.EmitParam(f.Name, b.Raw, fam,
				[]config.Value{config.VStr(lines[0].Str), config.VInt(int64(len(lines)))},
				map[string]config.Value{"lines": config.VSet(lines...)}, bl.Line)
		}
	}
}

func weIdentity(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Event Identity")
	if sec == nil {
		return
	}
	c.Emit(f.Name, b.Raw, "world_event",
		[]config.Value{config.VStr("event.spirit_surge")},
		map[string]config.Value{
			"event_id": config.VStr("event.spirit_surge"),
			"schedule": config.VStr("every whole UTC hour"),
			"duration": config.VStr("15m"),
		}, sec.Line)
	var regions, elements []config.Value
	for _, bl := range sec.Content {
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := regexp.MustCompile(`^[0-9]\s+(zone\.[a-z0-9_]+)$`).FindStringSubmatch(l); m != nil {
				regions = append(regions, config.VStr(m[1]))
			}
			if m := regexp.MustCompile(`^[0-9]\s+(KIM|MOC|THUY|HOA|THO)$`).FindStringSubmatch(l); m != nil {
				elements = append(elements, config.VStr(m[1]))
			}
			if strings.HasPrefix(l, "pairs =") {
				c.EmitParam(f.Name, b.Raw, "region_pairs",
					[]config.Value{config.VStr("pairs")},
					map[string]config.Value{"pairs": config.VStr(l)}, bl.Line)
			}
		}
	}
	c.EmitParam(f.Name, b.Raw, "region_order",
		[]config.Value{config.VStr("region")},
		map[string]config.Value{"order": config.VList(regions...)}, sec.Line)
	c.EmitParam(f.Name, b.Raw, "element_order",
		[]config.Value{config.VStr("element")},
		map[string]config.Value{"order": config.VList(elements...)}, sec.Line)
	c.consumed(f, b)
}

func weAccessTable(c *Ctx, f *File, b *SourceBinding) {
	for _, sec := range f.Root.Children {
		if sec.Title != "Event Identity" {
			continue
		}
		for _, bl := range sec.Content {
			if bl.Kind != BlockTable || !hasHeaders(bl, "H mod 5") {
				continue
			}
			for ri := range bl.Cells {
				row := bl.Cells[ri]
				hm, _ := (TypeSpec{Name: "int"}).ParseValue(cellAt(row, 0).Scalar())
				var regs []config.Value
				for _, t := range strings.Split(cellAt(row, 1).Scalar(), ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						regs = append(regs, config.VStr(t))
					}
				}
				c.Emit(f.Name, b.Raw, "surge_regions",
					[]config.Value{hm},
					map[string]config.Value{
						"h_mod_5":        hm,
						"active_regions": config.VList(regs...),
						"act1_index":     config.VStr(cellAt(row, 2).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

func weFieldOrder(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Eligible Field Order")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		if bl.Kind != BlockTable {
			continue
		}
		for ri := range bl.Cells {
			row := bl.Cells[ri]
			region := cellAt(row, 0).Scalar()
			for slot := 1; slot <= 3 && slot < len(row); slot++ {
				c.Emit(f.Name, b.Raw, "surge_field",
					[]config.Value{config.VStr(region), config.VInt(int64(slot - 1))},
					map[string]config.Value{
						"region": config.VStr(region),
						"slot":   config.VInt(int64(slot - 1)),
						"map_id": config.VStr(cellAt(row, slot).Scalar()),
					}, row[0].Line)
			}
		}
	}
	c.consumed(f, b)
}

var wePayloadRe = regexp.MustCompile(`^(KIM|MOC|THUY|HOA|THO)\s*->\s*(.+)$`)

func weVariant(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Surge Variant")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if m := wePayloadRe.FindStringSubmatch(l); m != nil {
				c.Emit(f.Name, b.Raw, "surge_element_attack",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"element": config.VStr(m[1]),
						"payload": config.VStr(strings.TrimSpace(m[2])),
					}, bl.Line+1+j)
				continue
			}
			kv := chestFieldRe.FindStringSubmatch(l)
			if kv != nil {
				c.EmitParam(f.Name, b.Raw, "surge_variant_stat",
					[]config.Value{config.VStr(kv[1])},
					map[string]config.Value{
						"field": config.VStr(kv[1]),
						"value": config.VStr(strings.TrimSpace(kv[2])),
					}, bl.Line+1+j)
			} else {
				c.EmitParam(f.Name, b.Raw, "surge_variant_rule",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func weSpawnGroups(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Temporary Spawn Groups")
	if sec == nil {
		return
	}
	c.EmitParam(f.Name, b.Raw, "surge_spawn_budget",
		[]config.Value{config.VStr("temp_groups")},
		map[string]config.Value{
			"max_groups": config.VInt(2),
			"max_alive":  config.VInt(4),
		}, sec.Line)
	c.consumed(f, b)
}

func weEliteChain(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Elite Event Chain")
	if sec == nil {
		return
	}
	waveRe := regexp.MustCompile("`(event\\.spirit_surge\\.wave\\.[0-9]+)`")
	stepRe := regexp.MustCompile("^-\\s*(defeat|activate)\\s*`([0-9]+)`\\s*(.*)$")
	wave := 0
	for _, bl := range sec.Content {
		for j, l := range bl.Prose {
			l = strings.TrimSpace(l)
			if m := waveRe.FindStringSubmatch(l); m != nil {
				wave++
				c.Emit(f.Name, b.Raw, "surge_chain_step",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"step_id": config.VStr(m[1]),
						"wave":    config.VInt(int64(wave)),
					}, bl.Line+1+j)
				continue
			}
			if m := stepRe.FindStringSubmatch(l); m != nil {
				n, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				c.Emit(f.Name, b.Raw, "surge_chain_requirement",
					[]config.Value{config.VStr("event.spirit_surge.wave.0" + string(rune('0'+wave))), config.VInt(int64(j))},
					map[string]config.Value{
						"verb":   config.VStr(strings.ToUpper(m[1])),
						"count":  n,
						"target": config.VStr(strings.TrimSpace(m[3])),
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func weParticipation(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Participation")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if strings.Contains(l, ">=") || strings.Contains(l, ":") {
				kv := regexp.MustCompile(`^([a-z_ ]+):\s*(.+)$`).FindStringSubmatch(l)
				if kv != nil {
					c.EmitParam(f.Name, b.Raw, "contribution_rule",
						[]config.Value{config.VStr(strings.TrimSpace(kv[1]))},
						map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
					continue
				}
			}
			c.EmitParam(f.Name, b.Raw, "contribution_rule",
				[]config.Value{config.VStr(l)},
				map[string]config.Value{"rule": config.VStr(l)}, bl.Line+1+j)
		}
	}
	c.consumed(f, b)
}

func weRewards(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Rewards")
	if sec == nil {
		return
	}
	expRe := regexp.MustCompile(`^Act\s+([IVX]+)\s+([0-9]+)`)
	for _, bl := range sec.Content {
		for j, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if m := expRe.FindStringSubmatch(l); m != nil {
				v, _ := (TypeSpec{Name: "int"}).ParseValue(m[2])
				c.Emit(f.Name, b.Raw, "surge_exp",
					[]config.Value{config.VStr(m[1])},
					map[string]config.Value{
						"act":          config.VStr(m[1]),
						"exp":          v,
						"key_template": config.VStr("surge.completion.exp.<utc_hour>.<character_id>"),
					}, bl.Line+1+j)
				continue
			}
			if strings.HasPrefix(l, "drop.event.") {
				parts := strings.Fields(l)
				c.Emit(f.Name, b.Raw, "surge_reward",
					[]config.Value{config.VStr(parts[0])},
					map[string]config.Value{
						"drop_table_id": config.VStr(parts[0]),
						"detail":        config.VStr(l),
					}, bl.Line+1+j)
			}
		}
	}
	c.consumed(f, b)
}

func weRestart(c *Ctx, f *File, b *SourceBinding) {
	sec := f.Root.SectionAt("Restart / Idempotency")
	if sec == nil {
		return
	}
	for _, bl := range sec.Content {
		for _, l := range bl.FLines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "spirit_surge.") {
				c.EmitParam(f.Name, b.Raw, "surge_instance_key",
					[]config.Value{config.VStr(l)},
					map[string]config.Value{"key_template": config.VStr(l)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}

func weValidation(c *Ctx, f *File, b *SourceBinding) {
	for _, title := range []string{"Validation", "Invariants"} {
		sec := f.Root.SectionAt(title)
		if sec == nil {
			continue
		}
		for _, bl := range sec.Content {
			var rules []config.Value
			for _, l := range bl.FLines {
				l = strings.TrimSpace(l)
				if l != "" {
					rules = append(rules, config.VStr(l))
				}
			}
			for _, l := range bl.Prose {
				l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "-"))
				if l != "" && l != "Reject:" {
					rules = append(rules, config.VStr(l))
				}
			}
			if len(rules) > 0 {
				c.EmitParam(f.Name, b.Raw, "world_event_rule",
					[]config.Value{config.VStr(title)},
					map[string]config.Value{"rules": config.VSet(rules...)}, bl.Line)
			}
		}
	}
	c.consumed(f, b)
}
