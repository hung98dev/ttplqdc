package main

import (
	"regexp"
	"strconv"
	"strings"

	"thinhthan/internal/config"
)

// compileValidationRules — generic driver for balance_validation.md and
// integration_validation.md. Each registry binding declares a `gate <id>` or
// `rules <id>` output; the driver extracts every fence line, table row and
// bullet in the bound sections as assertion lines on one validation_rule
// record per (gate, section), plus validation_vector rows for tables.
func compileValidationRules(c *Ctx, f *File, r *Registry) {
	for _, b := range r.Bindings {
		gateID, kind := validationKey(b.Output)
		if gateID == "" {
			gateID = "validation." + sanitize(b.Raw)
		}
		secs := bindingSections(c, f, b)
		if len(secs) == 0 {
			c.Diags.Addf(config.DiagSourceSchemaMissing, f.Path, b.Line,
				"validation binding %q resolves no sections", b.Raw)
			continue
		}
		secOrd := 0
		for _, sec := range secs {
			secOrd++
			var rules []config.Value
			ord := 0
			for _, bl := range sec.Content {
				for _, l := range bl.FLines {
					l = strings.TrimSpace(l)
					if l == "" {
						continue
					}
					ord++
					rules = append(rules, config.VStr(l))
				}
				for _, l := range bl.Prose {
					l = strings.TrimSpace(l)
					if l == "" || l == "Reject:" || l == "Reject when:" ||
						l == "Current expected" || strings.HasPrefix(l, "Release guardrail") ||
						strings.HasPrefix(l, "##") {
						continue
					}
					// keep bullet assertion lines only
					if strings.HasPrefix(l, "-") || strings.HasPrefix(l, "*") ||
						regexp.MustCompile(`^[0-9]+\.`).MatchString(l) {
						ord++
						rules = append(rules, config.VStr(strings.TrimLeft(l, "-* ")))
					}
				}
				if bl.Kind == BlockTable && len(bl.Cells) > 0 {
					for ri := range bl.Cells {
						row := bl.Cells[ri]
						ord++
						var cells []string
						for _, cell := range row {
							cells = append(cells, strings.ReplaceAll(cell.Scalar(), "`", ""))
						}
						rules = append(rules, config.VStr(strings.Join(cells, " | ")))
						key := config.VStr(gateID + "." + cellAt(row, 0).Scalar())
						fields := map[string]config.Value{
							"gate":    config.VStr(gateID),
							"row_key": config.VStr(strings.ReplaceAll(cellAt(row, 0).Scalar(), "`", "")),
						}
						for ci, h := range bl.Headers {
							if ci < len(row) {
								fields[fieldName(h)] = config.VStr(strings.ReplaceAll(row[ci].Scalar(), "`", ""))
							}
						}
						c.Emit(f.Name, b.Raw, "validation_vector",
							[]config.Value{key}, fields, row[0].Line)
					}
				}
			}
			// subsections count too (## Solo, ## Five-player, ...)
			for _, sub := range sec.Children {
				for _, bl := range sub.Content {
					for _, l := range bl.FLines {
						l = strings.TrimSpace(l)
						if l == "" {
							continue
						}
						ord++
						rules = append(rules, config.VStr(sub.Title+": "+l))
					}
					for _, l := range bl.Prose {
						l = strings.TrimSpace(l)
						if strings.HasPrefix(l, "-") || regexp.MustCompile(`^[0-9]+\.`).MatchString(l) {
							ord++
							rules = append(rules, config.VStr(sub.Title+": "+strings.TrimLeft(l, "- ")))
						}
					}
					if bl.Kind == BlockTable && len(bl.Cells) > 0 {
						for ri := range bl.Cells {
							row := bl.Cells[ri]
							var cells []string
							for _, cell := range row {
								cells = append(cells, strings.ReplaceAll(cell.Scalar(), "`", ""))
							}
							ord++
							rules = append(rules, config.VStr(sub.Title+": "+strings.Join(cells, " | ")))
							key := config.VStr(gateID + "." + cellAt(row, 0).Scalar())
							fields := map[string]config.Value{
								"gate":    config.VStr(gateID),
								"row_key": config.VStr(strings.ReplaceAll(cellAt(row, 0).Scalar(), "`", "")),
							}
							for ci, h := range bl.Headers {
								if ci < len(row) {
									fields[fieldName(h)] = config.VStr(strings.ReplaceAll(row[ci].Scalar(), "`", ""))
								}
							}
							c.Emit(f.Name, b.Raw, "validation_vector",
								[]config.Value{key}, fields, row[0].Line)
						}
					}
				}
			}
			if len(rules) == 0 {
				continue
			}
			c.Emit(f.Name, b.Raw, "validation_rule",
				[]config.Value{config.VStr(gateID), config.VInt(int64(secOrd))},
				map[string]config.Value{
					"gate_id":    config.VStr(gateID),
					"kind":       config.VStr(kind),
					"section":    config.VStr(sec.Title),
					"assertions": config.VSet(rules...),
				}, sec.Line)
		}
		c.consumed(f, b)
	}
}

var validKeyRe = regexp.MustCompile("`([a-z_]+\\.[a-z0-9_.]+)`")

func validationKey(output string) (id, kind string) {
	m := validKeyRe.FindStringSubmatch(output)
	if m != nil {
		id = m[1]
	}
	switch {
	case strings.Contains(output, "gate"):
		kind = "GATE"
	case strings.Contains(output, "rules"):
		kind = "RULES"
	default:
		kind = "VECTOR"
	}
	return id, kind
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func compileBalanceValidation(c *Ctx, f *File, r *Registry)     { compileValidationRules(c, f, r) }
func compileIntegrationValidation(c *Ctx, f *File, r *Registry) { compileValidationRules(c, f, r) }

var _ = strconv.Itoa
