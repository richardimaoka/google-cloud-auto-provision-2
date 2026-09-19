package main

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// The placeholders for the members of an IAM policy, handed out in the order the
// real values first appear.
var (
	userEmails          = []string{"alice@example.com", "bob@example.com", "carol@example.com", "dave@example.com"}
	groupEmails         = []string{"group-1@example.com", "group-2@example.com", "group-3@example.com"}
	serviceAccountNames = []string{"my-service-account", "my-service-account-2", "my-service-account-3"}
)

// iamEtagValue is the placeholder for the etag of an IAM policy, which is
// base64 and has no "W/" like the etag of a project.
const iamEtagValue = "BwAAAAAAAAA="

// memberKinds are the forms of a member this tool knows: "kind:value". The
// members allUsers and allAuthenticatedUsers have no value.
var memberKinds = []string{"user", "group", "domain", "serviceAccount", "projectOwner", "projectEditor", "projectViewer"}

var (
	memberRE  = regexp.MustCompile(`"(` + strings.Join(memberKinds, "|") + `):([^"]*)"`)
	iamEtagRE = regexp.MustCompile(`("etag"\s*:\s*")[A-Za-z0-9+/_-]+={0,2}(")`)

	// The names of the service agents and the default service accounts that hold
	// a project number.
	agentAccountRE   = regexp.MustCompile(`^service-(\d+)$`)
	computeAccountRE = regexp.MustCompile(`^(\d+)-compute$`)
)

const userManagedDomainSuffix = ".iam.gserviceaccount.com"

var errUnknownServiceAccount = errors.New("a service account is in a form this tool doesn't know, replace it by hand")

// A condition is a CEL expression with a title and a description. The tool
// leaves them as they are, so it lets through only an expression that has no
// value of the project in it: one that uses just these attributes of the request
// and the resource. An attribute like resource.name can hold the name of a
// resource, so it is not one of them.
var knownConditionAttributes = []string{"request.time", "resource.type", "resource.service"}

var (
	stringLiteralRE = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	// attributeRE finds the attributes in an expression, where the string
	// literals are left out.
	attributeRE = regexp.MustCompile(`\b(?:request|resource|api|destination|origin)\.[A-Za-z_][A-Za-z0-9_.]*`)
)

var errUnknownCondition = errors.New("a binding has a condition this tool doesn't know, replace it by hand")

func checkCondition(v any) error {
	condition, ok := v.(map[string]any)
	if !ok {
		return errUnknownCondition
	}
	for key, value := range condition {
		if _, isString := value.(string); !isString || (key != "expression" && key != "title" && key != "description") {
			return errUnknownCondition
		}
	}
	expression, _ := condition["expression"].(string)
	if expression == "" {
		return errUnknownCondition
	}
	for _, attribute := range attributeRE.FindAllString(stringLiteralRE.ReplaceAllString(expression, `""`), -1) {
		if !isKnownConditionAttribute(attribute) {
			return errUnknownCondition
		}
	}
	return nil
}

func isKnownConditionAttribute(attribute string) bool {
	for _, known := range knownConditionAttributes {
		if attribute == known || strings.HasPrefix(attribute, known+".") {
			return true
		}
	}
	return false
}

// checkBindings stops on a binding that has something this tool does not
// replace: a condition it does not know (see checkCondition), or a member of a
// form it does not know, like a deleted member with its ID or a workload
// identity.
func checkBindings(top map[string]any) error {
	bindings, _ := top["bindings"].([]any)
	for _, b := range bindings {
		binding, _ := b.(map[string]any)
		if condition, ok := binding["condition"]; ok {
			if err := checkCondition(condition); err != nil {
				return err
			}
		}
		members, _ := binding["members"].([]any)
		for _, m := range members {
			member, _ := m.(string)
			if !isKnownMember(member) {
				return errors.New("a member is in a form this tool doesn't know, replace it by hand")
			}
		}
	}
	return nil
}

func isKnownMember(member string) bool {
	if member == "allUsers" || member == "allAuthenticatedUsers" {
		return true
	}
	kind, _, ok := strings.Cut(member, ":")
	return ok && slices.Contains(memberKinds, kind)
}

// anonymizeMembers replaces the members of IAM bindings in text: the emails of
// users and groups, the domain, and the service accounts. The project numbers in
// a service account come from numbers, so they match the ones in "projects/<n>".
// The project IDs it meets are added to found, and are replaced later along with
// the others.
func anonymizeMembers(text string, numbers *allocator, found map[string]bool) (string, error) {
	users := newAllocator("user email addresses", userEmails)
	groups := newAllocator("group email addresses", groupEmails)
	accounts := newAllocator("service account names", serviceAccountNames)

	var failure error
	out := memberRE.ReplaceAllStringFunc(text, func(match string) string {
		if failure != nil {
			return match
		}
		parts := memberRE.FindStringSubmatch(match)
		kind, value := parts[1], parts[2]

		var replaced string
		var err error
		switch kind {
		case "user":
			replaced, err = users.get(value)
		case "group":
			replaced, err = groups.get(value)
		case "domain":
			replaced = "example.com"
		case "serviceAccount":
			replaced, err = anonymizeServiceAccount(value, numbers, accounts, found)
		default: // projectOwner, projectEditor and projectViewer hold a project ID.
			found[value] = true
			replaced = value
		}
		if err != nil {
			failure = err
			return match
		}
		return `"` + kind + ":" + replaced + `"`
	})
	return out, failure
}

// anonymizeServiceAccount replaces what is real in the email of a service
// account. The service agents ("service-<number>@<agent>.iam.gserviceaccount.com")
// keep their domain, which belongs to Google.
func anonymizeServiceAccount(email string, numbers, accounts *allocator, found map[string]bool) (string, error) {
	local, domain, ok := strings.Cut(email, "@")
	if !ok {
		return "", errUnknownServiceAccount
	}
	switch {
	case domain == "developer.gserviceaccount.com":
		m := computeAccountRE.FindStringSubmatch(local)
		if m == nil {
			return "", errUnknownServiceAccount
		}
		n, err := numbers.get(m[1])
		return n + "-compute@" + domain, err
	case domain == "cloudservices.gserviceaccount.com":
		if !isDigits(local) {
			return "", errUnknownServiceAccount
		}
		n, err := numbers.get(local)
		return n + "@" + domain, err
	case domain == "appspot.gserviceaccount.com":
		found[local] = true
		return email, nil
	case strings.HasSuffix(domain, userManagedDomainSuffix):
		if m := agentAccountRE.FindStringSubmatch(local); m != nil {
			n, err := numbers.get(m[1])
			return "service-" + n + "@" + domain, err
		}
		found[strings.TrimSuffix(domain, userManagedDomainSuffix)] = true
		name, err := accounts.get(local)
		return name + "@" + domain, err
	}
	return "", errUnknownServiceAccount
}

// isPlaceholderEmail reports whether email is one of the service account
// addresses that anonymizeMembers hands out. They are not at example.com, but
// they hold no real value, so they may be left.
func isPlaceholderEmail(email string) bool {
	local, domain, _ := strings.Cut(email, "@")
	switch domain {
	case "developer.gserviceaccount.com":
		n, ok := strings.CutSuffix(local, "-compute")
		return ok && isPlaceholderNumber(n)
	case "cloudservices.gserviceaccount.com":
		return isPlaceholderNumber(local)
	case "appspot.gserviceaccount.com":
		return slices.Contains(projectIDs, local)
	}
	project, ok := strings.CutSuffix(domain, userManagedDomainSuffix)
	if !ok {
		return false
	}
	if n, ok := strings.CutPrefix(local, "service-"); ok {
		return isPlaceholderNumber(n)
	}
	return slices.Contains(projectIDs, project) && slices.Contains(serviceAccountNames, local)
}

func isPlaceholderNumber(s string) bool {
	return slices.Contains(ownProjectNumbers, s) || slices.Contains(otherProjectNumbers, s)
}

// conditionHashes are the placeholders for the hash after "_withcond_" in a role
// name. A conditional binding read without policy version 3 has its condition
// left out, and the role name gets the hash of the condition instead.
var conditionHashes = []string{"aaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccc"}

var conditionHashRE = regexp.MustCompile(`(_withcond_)[0-9a-f]{8,}`)

// anonymizeConditionHashes replaces the hashes in the role names of conditional
// bindings. The same hash gets the same placeholder.
func anonymizeConditionHashes(text string) (string, error) {
	hashes := newAllocator("condition hashes", conditionHashes)
	var failure error
	out := conditionHashRE.ReplaceAllStringFunc(text, func(match string) string {
		hash := strings.TrimPrefix(match, "_withcond_")
		placeholder, err := hashes.get(hash)
		if err != nil {
			failure = err
			return match
		}
		return "_withcond_" + placeholder
	})
	return out, failure
}

// replaceIAMEtag replaces the etag of an IAM policy.
func replaceIAMEtag(text string) string {
	return iamEtagRE.ReplaceAllString(text, "${1}"+iamEtagValue+"${2}")
}
