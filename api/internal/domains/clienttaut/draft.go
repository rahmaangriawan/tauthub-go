package clienttaut

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// parseProfilerDraft: parity profilerDraftSchema di schema.ts (lenient
// autosave untuk TautProfileDraftPut — field boleh string kosong).

var dqKeyPattern = regexp.MustCompile(`^[a-z0-9_]{2,40}$`)

func parseProfilerDraft(body map[string]any) (profilerAnalysisForm, error) {
	var out profilerAnalysisForm

	if raw, ok, err := fieldStr(body, "name"); err != nil {
		return out, err
	} else if ok {
		if utf8.RuneCountInString(raw) > 200 {
			return out, zodTooBigString(200)
		}
		out.Name = strings.TrimSpace(raw)
	}

	if m, has, err := fieldMap(body, "business_profile"); err != nil {
		return out, err
	} else if has {
		for _, key := range []string{"business", "industry", "audience", "usp", "website"} {
			if raw, ok, err := fieldStr(m, key); err != nil {
				return out, err
			} else if ok {
				limit := 200
				switch key {
				case "industry":
					limit = 120
				case "audience", "usp":
					limit = 500
				case "website":
					limit = 300
				}
				if utf8.RuneCountInString(raw) > limit {
					return out, zodTooBigString(limit)
				}
				trimmed := strings.TrimSpace(raw)
				switch key {
				case "business":
					out.BusinessProfile.Business = trimmed
				case "industry":
					out.BusinessProfile.Industry = trimmed
				case "audience":
					out.BusinessProfile.Audience = trimmed
				case "usp":
					out.BusinessProfile.USP = trimmed
				case "website":
					out.BusinessProfile.Website = trimmed
				}
			}
		}
	}

	// objective: union [enum, record] .nullable() — enum valid bekukan string,
	// record/object lama tetap diterima apa adanya.
	if v, ok := body["objective"]; ok && v != nil {
		if s, isStr := v.(string); isStr {
			out.Objective = s
		} else {
			out.Objective = v
		}
	}

	if m, has, err := fieldMap(body, "audience"); err != nil {
		return out, err
	} else if has {
		if arr, ok, err := fieldArray(m, "selected"); err != nil {
			return out, err
		} else if ok {
			if len(arr) > 20 {
				return out, zodTooBigArray(20)
			}
			for _, item := range arr {
				s, isStr := item.(string)
				if !isStr {
					return out, zodInvalidType("string", receivedType(item))
				}
				if utf8.RuneCountInString(s) < 1 {
					return out, zodTooSmallString(1)
				}
				out.Audience.Selected = append(out.Audience.Selected, strings.TrimSpace(s))
			}
		}
		if raw, ok, err := fieldStr(m, "custom"); err != nil {
			return out, err
		} else if ok {
			if utf8.RuneCountInString(raw) > 200 {
				return out, zodTooBigString(200)
			}
			out.Audience.Custom = strings.TrimSpace(raw)
		}
	}

	for _, key := range []string{"spokesperson", "spokepersons"} {
		if key == "spokesperson" {
			if m, has, err := fieldMap(body, "spokesperson"); err != nil {
				return out, err
			} else if has {
				sp, err := parseSpokespersonLenient(m)
				if err != nil {
					return out, err
				}
				out.Spokesperson = sp
			}
			continue
		}
		if arr, ok, err := fieldArray(body, "spokepersons"); err != nil {
			return out, err
		} else if ok {
			for _, item := range arr {
				obj, isObj := item.(map[string]any)
				if !isObj {
					return out, zodInvalidType("object", receivedType(item))
				}
				sp, err := parseSpokespersonLenient(obj)
				if err != nil {
					return out, err
				}
				out.Spokepersons = append(out.Spokepersons, sp)
			}
		}
	}

	if m, has, err := fieldMap(body, "guardrails"); err != nil {
		return out, err
	} else if has {
		if arr, ok, err := fieldArray(m, "rules"); err != nil {
			return out, err
		} else if ok {
			if len(arr) > 50 {
				return out, zodTooBigArray(50)
			}
			for _, item := range arr {
				s, isStr := item.(string)
				if !isStr {
					return out, zodInvalidType("string", receivedType(item))
				}
				if utf8.RuneCountInString(s) < 1 {
					return out, zodTooSmallString(1)
				}
				if utf8.RuneCountInString(s) > 2000 {
					return out, zodTooBigString(2000)
				}
				out.Guardrails.Rules = append(out.Guardrails.Rules, strings.TrimSpace(s))
			}
		}
	}

	if m, has, err := fieldMap(body, "backlink"); err != nil {
		return out, err
	} else if has {
		out.Backlink = backlinkStrategy{SmartVariations: true}
		if v, ok, err := fieldBool(m, "enabled"); err != nil {
			return out, err
		} else if ok {
			out.Backlink.Enabled = v
		}
		if raw, ok, err := fieldStr(m, "target_url"); err != nil {
			return out, err
		} else if ok {
			if utf8.RuneCountInString(raw) > 500 {
				return out, zodTooBigString(500)
			}
			out.Backlink.TargetURL = strings.TrimSpace(raw)
		}
		if raw, ok, err := fieldStr(m, "anchor_text"); err != nil {
			return out, err
		} else if ok {
			if utf8.RuneCountInString(raw) > 120 {
				return out, zodTooBigString(120)
			}
			out.Backlink.AnchorText = strings.TrimSpace(raw)
		}
		if v, ok, err := fieldBool(m, "smart_variations"); err != nil {
			return out, err
		} else if ok {
			out.Backlink.SmartVariations = v
		}
	}

	if arr, ok, err := fieldArray(body, "channels"); err != nil {
		return out, err
	} else if ok {
		if len(arr) > 2 {
			return out, zodTooBigArray(2)
		}
		for _, item := range arr {
			obj, isObj := item.(map[string]any)
			if !isObj {
				return out, zodInvalidType("object", receivedType(item))
			}
			raw, has, err := fieldStr(obj, "channel")
			if err != nil {
				return out, err
			}
			if !has {
				return out, zodInvalidType("string", "undefined")
			}
			channel, err := validateEnum(raw, "mainstream", "blogger")
			if err != nil {
				return out, err
			}
			sel := selectedChannel{Channel: channel}
			if rawAngle, ok, err := fieldStr(obj, "angle_slug"); err != nil {
				return out, err
			} else if ok {
				if utf8.RuneCountInString(rawAngle) > 120 {
					return out, zodTooBigString(120)
				}
				sel.AngleSlug = strings.TrimSpace(rawAngle)
			}
			out.Channels = append(out.Channels, sel)
		}
	}

	if m, has, err := fieldMap(body, "materials"); err != nil {
		return out, err
	} else if has {
		out.MaterialsMode = "dynamic"
		if raw, ok, err := fieldStr(m, "mode"); err != nil {
			return out, err
		} else if ok {
			mode, err := validateEnum(raw, "upload", "dynamic")
			if err != nil {
				return out, err
			}
			out.MaterialsMode = mode
		}
		if arr, ok, err := fieldArray(m, "file_ids"); err != nil {
			return out, err
		} else if ok {
			if len(arr) > 10 {
				return out, zodTooBigArray(10)
			}
			for _, item := range arr {
				s, isStr := item.(string)
				if !isStr {
					return out, zodInvalidType("string", receivedType(item))
				}
				if utf8.RuneCountInString(strings.TrimSpace(s)) < 1 {
					return out, zodTooSmallString(1)
				}
				out.MaterialsFileIDs = append(out.MaterialsFileIDs, strings.TrimSpace(s))
			}
		}
		if dyn, ok, err := fieldMap(m, "dynamic"); err != nil {
			return out, err
		} else if ok {
			out.MaterialsDynamic = map[string]any{}
			for k, v := range dyn {
				if s, isStr := v.(string); isStr {
					if utf8.RuneCountInString(s) > 2000 {
						return out, zodTooBigString(2000)
					}
					out.MaterialsDynamic[k] = strings.TrimSpace(s)
				} else if rec, isRec := v.(map[string]any); isRec {
					nested := map[string]any{}
					for nk, nv := range rec {
						s, isStr := nv.(string)
						if !isStr {
							return out, zodInvalidType("string", receivedType(nv))
						}
						if utf8.RuneCountInString(s) > 2000 {
							return out, zodTooBigString(2000)
						}
						nested[nk] = strings.TrimSpace(s)
					}
					out.MaterialsDynamic[k] = nested
				} else {
					return out, zodInvalidType("string", receivedType(v))
				}
			}
		}
	}

	if m, has, err := fieldMap(body, "budget"); err != nil {
		return out, err
	} else if has {
		out.Budget = budgetStrategy{Mode: "efficient"}
		if raw, ok, err := fieldStr(m, "mode"); err != nil {
			return out, err
		} else if ok {
			mode, err := validateEnum(raw, "efficient", "optimal")
			if err != nil {
				return out, err
			}
			out.Budget.Mode = mode
		}
		if v, ok, err := fieldNum(m, "amount"); err != nil {
			return out, err
		} else if ok {
			if v != float64(int(v)) {
				return out, errUnprocessable("Invalid input: expected int, received number")
			}
			if v < 0 {
				return out, zodTooSmallNumber("0")
			}
			out.Budget.Amount = int(v)
		}
	}
	return out, nil
}

func parseSpokespersonLenient(m map[string]any) (spokesperson, error) {
	var out spokesperson
	for _, key := range []string{"name", "role", "quote"} {
		raw, ok, err := fieldStr(m, key)
		if err != nil {
			return out, err
		}
		if !ok {
			continue
		}
		limit := 200
		if key == "quote" {
			limit = 500
		}
		if utf8.RuneCountInString(raw) > limit {
			return out, zodTooBigString(limit)
		}
		trimmed := strings.TrimSpace(raw)
		switch key {
		case "name":
			out.Name = trimmed
		case "role":
			out.Role = trimmed
		case "quote":
			out.Quote = trimmed
		}
	}
	return out, nil
}
