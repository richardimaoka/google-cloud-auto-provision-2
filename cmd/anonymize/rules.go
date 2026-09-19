package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The placeholders handed out to the distinct real values of one response, in
// the order the values first appear. The first of each is what the samples in
// gcp/sampledata already use. The others are for responses that have more than
// one, and can be extended.
var (
	// Project numbers of the caller's own projects, like the project a
	// successful response is about.
	ownProjectNumbers = []string{"123456789012", "234567890123", "345678901234"}
	// Project numbers in an error response. Those are projects the caller can't
	// use, so they are told apart from the caller's own.
	otherProjectNumbers = []string{"987654321098", "876543210987", "765432109876"}
	organizationNumbers = []string{"100000000000", "200000000000", "300000000000"}
	projectIDs          = []string{"my-project-123", "my-project-456", "my-project-789"}
)

const (
	operationNumber = "1234567890123456789"
	etagValue       = "0123456789abcdef"
	zeroUUID        = "00000000-0000-0000-0000-000000000000"
)

// allocator hands out a placeholder for each distinct real value.
type allocator struct {
	what         string
	placeholders []string
	assigned     map[string]string
}

func newAllocator(what string, placeholders []string) *allocator {
	return &allocator{what: what, placeholders: placeholders, assigned: map[string]string{}}
}

func (a *allocator) get(real string) (string, error) {
	if p, ok := a.assigned[real]; ok {
		return p, nil
	}
	if len(a.assigned) >= len(a.placeholders) {
		return "", fmt.Errorf("more than %d distinct %s, add placeholders in rules.go", len(a.placeholders), a.what)
	}
	p := a.placeholders[len(a.assigned)]
	a.assigned[real] = p
	return p, nil
}

// identifierRE finds the values that hold identifiers. Groups: 1 project
// number, 2 organization number, 3 the name of an operation up to its number,
// 4 an encoded error ID and 5 its padding. An etag has no group.
//
// An encoded error ID starts with "CiQ" when its UUID starts with a digit, and
// with "CiR" when it starts with a letter.
var identifierRE = regexp.MustCompile(
	`projects/(\d+)` +
		`|organizations/(\d+)` +
		`|(operations/[A-Za-z_]+(?:\.[A-Za-z_]+)*\.)\d+` +
		`|W/\\"[0-9a-f]+\\"` +
		`|(Ci[QR][A-Za-z0-9+/_-]+)((?:%3D|=)*)`)

var emailRE = regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9.-]+\.[A-Za-z]{2,})`)

// anonymize replaces the real identifiers in a recorded API response with the
// placeholders, and returns it as indented JSON. It fails, and writes nothing,
// if the input is not JSON, if an identifier is in a form it doesn't know, or
// if an email address is left that is not at example.com.
func anonymize(data []byte) ([]byte, error) {
	text := strings.NewReplacer("\u200b", "", "\ufeff", "").Replace(string(data))
	text = strings.TrimSpace(text)

	var doc any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	top, _ := doc.(map[string]any)
	_, isError := top["error"]

	// Project IDs can't be found by their shape. They are the values of
	// "projectId", and the "subject" of a violation.
	found := map[string]bool{}
	collectProjectIDs(doc, "", found)
	var ordered []string
	for id := range found {
		if strings.Contains(text, id) {
			ordered = append(ordered, id)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return strings.Index(text, ordered[i]) < strings.Index(text, ordered[j]) })
	idMap := map[string]string{}
	for i, id := range ordered {
		if i >= len(projectIDs) {
			return nil, fmt.Errorf("more than %d distinct project IDs, add placeholders in rules.go", len(projectIDs))
		}
		idMap[id] = projectIDs[i]
	}
	// One pass, longest ID first, so a placeholder is never replaced again.
	var pairs []string
	for _, id := range sortedByLength(idMap) {
		pairs = append(pairs, id, idMap[id])
	}
	text = strings.NewReplacer(pairs...).Replace(text)

	numbers := newAllocator("project numbers", ownProjectNumbers)
	if isError {
		numbers = newAllocator("project numbers", otherProjectNumbers)
	}
	orgs := newAllocator("organization numbers", organizationNumbers)

	resource := func(res string) (string, error) {
		name, ok := strings.CutPrefix(res, "projects/")
		if !ok {
			return res, nil
		}
		if isDigits(name) {
			n, err := numbers.get(name)
			return "projects/" + n, err
		}
		if p, ok := idMap[name]; ok {
			return "projects/" + p, nil
		}
		return res, nil
	}

	var out strings.Builder
	var masked strings.Builder // the same text with encoded error IDs left out, to look for leftovers
	last := 0
	for _, m := range identifierRE.FindAllStringSubmatchIndex(text, -1) {
		out.WriteString(text[last:m[0]])
		masked.WriteString(text[last:m[0]])
		match := text[m[0]:m[1]]
		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}

		switch {
		case group(1) != "":
			n, err := numbers.get(group(1))
			if err != nil {
				return nil, err
			}
			match = "projects/" + n
		case group(2) != "":
			n, err := orgs.get(group(2))
			if err != nil {
				return nil, err
			}
			match = "organizations/" + n
		case group(3) != "":
			match = group(3) + operationNumber
		case strings.HasPrefix(match, "W/"):
			match = `W/\"` + etagValue + `\"`
		case group(4) != "":
			blob, err := rewriteErrorID(group(4), group(5), zeroUUID, resource)
			if err != nil {
				return nil, err
			}
			out.WriteString(blob)
			masked.WriteString("<error id>")
			last = m[1]
			continue
		}
		out.WriteString(match)
		masked.WriteString(match)
		last = m[1]
	}
	out.WriteString(text[last:])
	masked.WriteString(text[last:])

	for _, m := range emailRE.FindAllStringSubmatchIndex(masked.String(), -1) {
		if domain := masked.String()[m[2]:m[3]]; !strings.EqualFold(domain, "example.com") {
			line := 1 + strings.Count(masked.String()[:m[0]], "\n")
			return nil, fmt.Errorf("an email address that is not @example.com is left on line %d, replace it by hand", line)
		}
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(out.String()), "", "  "); err != nil {
		return nil, fmt.Errorf("the result is not valid JSON: %w", err)
	}
	pretty.WriteByte('\n')
	return pretty.Bytes(), nil
}

func collectProjectIDs(v any, key string, found map[string]bool) {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			if s, ok := child.(string); ok && (k == "projectId" || (k == "subject" && key == "violations")) {
				found[s] = true
			}
			collectProjectIDs(child, k, found)
		}
	case []any:
		for _, child := range v {
			collectProjectIDs(child, key, found)
		}
	}
}

func sortedByLength(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// rewriteErrorID rewrites an encoded error ID: a request UUID and the resource
// the error is about, as `0x0a <len> uuid 0x12 <len> resource`, in base64.
// The UUID is set to uuid and the resource goes through resource. The padding
// (none, "=", or "%3D" in a URL) keeps its style. An ID of any other layout is
// an error, so a value is never left in it by mistake.
func rewriteErrorID(blob, pad, uuid string, resource func(string) (string, error)) (string, error) {
	enc := base64.StdEncoding
	if strings.ContainsAny(blob, "-_") {
		enc = base64.URLEncoding
	}
	raw, err := enc.DecodeString(blob + strings.Repeat("=", (4-len(blob)%4)%4))
	if err != nil {
		return "", fmt.Errorf("an error ID is not base64: %w", err)
	}

	unknown := fmt.Errorf("an error ID has a layout this tool doesn't know, replace it by hand")
	if len(raw) < 4 || raw[0] != 0x0a || raw[1] >= 0x80 {
		return "", unknown
	}
	n := int(raw[1])
	if len(raw) < 4+n || raw[2+n] != 0x12 || raw[3+n] >= 0x80 || len(raw) != 4+n+int(raw[3+n]) {
		return "", unknown
	}
	res, err := resource(string(raw[4+n:]))
	if err != nil {
		return "", err
	}

	rebuilt := []byte{0x0a, byte(len(uuid))}
	rebuilt = append(rebuilt, uuid...)
	rebuilt = append(rebuilt, 0x12, byte(len(res)))
	rebuilt = append(rebuilt, res...)

	encoded := enc.WithPadding(base64.NoPadding).EncodeToString(rebuilt)
	if pad != "" {
		unit := "="
		if strings.HasPrefix(pad, "%3D") {
			unit = "%3D"
		}
		encoded += strings.Repeat(unit, (3-len(rebuilt)%3)%3)
	}
	return encoded, nil
}
