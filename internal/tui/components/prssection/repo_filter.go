package prssection

import "strings"

func repoFromFilters(filters string) (string, bool) {
	var repo string
	for token := range strings.FieldsSeq(filters) {
		value, ok := strings.CutPrefix(token, "repo:")
		if !ok || value == "" {
			continue
		}
		if repo != "" {
			return "", false
		}
		repo = value
	}
	return repo, repo != ""
}

func (m *Model) repoFromFilters() (string, bool) {
	return repoFromFilters(m.GetFilters())
}
