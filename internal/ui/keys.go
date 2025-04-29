package ui

import "strings"

type shortcut struct {
	shortcut string
	label    string
}

type Shortcuts []shortcut

func NewShortcuts(shortcutAndDescriptions ...string) *Shortcuts {
	if len(shortcutAndDescriptions)%2 != 0 {
		panic("shortcuts must be in pairs")
	}
	shortcuts := make(Shortcuts, len(shortcutAndDescriptions)/2)
	for i := 0; i < len(shortcutAndDescriptions); i += 2 {
		shortcuts[i/2] = shortcut{
			shortcut: shortcutAndDescriptions[i],
			label:    shortcutAndDescriptions[i+1],
		}
	}
	return &shortcuts
}

func (s *Shortcuts) Add(sc, label string) *Shortcuts {
	*s = append(*s, shortcut{shortcut: sc, label: label})
	return s
}

func (s *Shortcuts) AddIf(b bool, sc, label string) *Shortcuts {
	if b {
		s.Add(sc, label)
	}
	return s
}

func (s *Shortcuts) Render(theme Theme) string {
	var bob strings.Builder
	for i, sc := range *s {
		if i != 0 {
			bob.WriteString(theme.MutedTextStyle.Render(", "))
		}
		bob.WriteString(sc.shortcut)
		bob.WriteString(" ")
		bob.WriteString(theme.MutedTextStyle.Render(sc.label))
	}
	return bob.String()
}
