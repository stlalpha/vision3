package menu

import "regexp"

var areaLightbarAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAreaAnsi(s string) string {
	return areaLightbarAnsiRe.ReplaceAllString(s, "")
}

// accessibleConf is a minimal conference descriptor for lightbar navigation.
type accessibleConf struct {
	id   int
	name string
}

// nextConf returns the ID of the conference after confID in the list, wrapping around.
func nextConf(confs []accessibleConf, confID int) int {
	if len(confs) == 0 {
		return confID
	}
	for i, c := range confs {
		if c.id == confID {
			return confs[(i+1)%len(confs)].id
		}
	}
	return confs[0].id
}

// prevConf returns the ID of the conference before confID in the list, wrapping around.
func prevConf(confs []accessibleConf, confID int) int {
	if len(confs) == 0 {
		return confID
	}
	for i, c := range confs {
		if c.id == confID {
			return confs[(i-1+len(confs))%len(confs)].id
		}
	}
	return confs[len(confs)-1].id
}
