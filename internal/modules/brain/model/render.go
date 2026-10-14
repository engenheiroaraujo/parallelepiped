package model

import (
	"fmt"
	"sort"
	"strings"
)

// GeneratedMarker identifies notes written by the import in their front
// matter. A note without it is never overwritten.
const GeneratedMarker = "gerado-por: parallelepiped"

// UserSectionHeading starts the part of a note that belongs to the user.
// Everything from this heading on survives every re-import.
const UserSectionHeading = "## Minhas anotações"

const userSectionPlaceholder = "<!-- Escreva abaixo. A reimportação nunca altera esta seção. -->"

// MaxListedSymbols bounds the exported symbols listed per unit.
const MaxListedSymbols = 40

// NoteFile is a generated note ready to be written.
type NoteFile struct {
	Path    string
	Content string
}

// RenderNotes turns a project into its notes: one index note, one hub per
// group and one note per unit. existingTitles are the lower-case titles of
// notes already in the vault (outside OutputDir), which unit titles must not
// reuse. The output is deterministic: same project, same bytes.
func RenderNotes(project Project, existingTitles map[string]bool) []NoteFile {
	units := append([]Unit(nil), project.Units...)
	sort.Slice(units, func(i, j int) bool { return units[i].Dir < units[j].Dir })

	groups := map[string][]Unit{}
	for _, unit := range units {
		groups[GroupOf(unit.Dir)] = append(groups[GroupOf(unit.Dir)], unit)
	}
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		if group != "" {
			groupNames = append(groupNames, group)
		}
	}
	sort.Strings(groupNames)

	reserved := map[string]bool{strings.ToLower(project.Name): true}
	for title := range existingTitles {
		reserved[title] = true
	}
	for _, group := range groupNames {
		reserved[strings.ToLower(HubTitle(group))] = true
	}
	dirs := make([]string, 0, len(units))
	for _, unit := range units {
		dirs = append(dirs, unit.Dir)
	}
	titles := UnitTitles(dirs, reserved)

	usedBy := map[string][]string{}
	for _, unit := range units {
		for _, dep := range unit.DependsOn {
			usedBy[dep] = append(usedBy[dep], unit.Dir)
		}
	}

	notes := []NoteFile{{
		Path:    NotePath("", project.Name),
		Content: renderIndex(project, groupNames, groups, titles),
	}}
	for _, group := range groupNames {
		notes = append(notes, NoteFile{
			Path:    NotePath(group, HubTitle(group)),
			Content: renderHub(project, group, groups[group], titles),
		})
	}
	for _, unit := range units {
		notes = append(notes, NoteFile{
			Path:    NotePath(GroupOf(unit.Dir), titles[unit.Dir]),
			Content: renderUnit(project, unit, titles, usedBy[unit.Dir]),
		})
	}
	return notes
}

func renderIndex(project Project, groupNames []string, groups map[string][]Unit, titles map[string]string) string {
	var b strings.Builder
	frontMatter(&b, ".")
	fmt.Fprintf(&b, "# %s\n\n", project.Name)
	b.WriteString("Mapa da estrutura do projeto, gerado a partir do código-fonte.\n\n")
	if project.ModulePath != "" {
		fmt.Fprintf(&b, "**Módulo Go:** `%s`  \n", project.ModulePath)
	}
	fmt.Fprintf(&b, "**Pastas com código:** %d · **Módulos:** %d\n\n", len(project.Units), len(groupNames))

	if len(groupNames) > 0 {
		b.WriteString("## Módulos\n\n")
		for _, group := range groupNames {
			fmt.Fprintf(&b, "- [[%s]] (%s)\n", HubTitle(group), plural(len(groups[group]), "pasta", "pastas"))
		}
		b.WriteString("\n")
	}
	if root := groups[""]; len(root) > 0 {
		b.WriteString("## Na raiz\n\n")
		for _, unit := range root {
			fmt.Fprintf(&b, "- [[%s]] — `%s`\n", titles[unit.Dir], unit.Dir)
		}
		b.WriteString("\n")
	}
	userSection(&b)
	return b.String()
}

func renderHub(project Project, group string, units []Unit, titles map[string]string) string {
	var b strings.Builder
	frontMatter(&b, group)
	fmt.Fprintf(&b, "# %s\n\n", HubTitle(group))
	fmt.Fprintf(&b, "Parte de [[%s]].\n\n", project.Name)
	b.WriteString("## Pastas\n\n")
	for _, unit := range units {
		fmt.Fprintf(&b, "- [[%s]] — `%s`", titles[unit.Dir], unit.Dir)
		if unit.Doc != "" {
			fmt.Fprintf(&b, ": %s", unit.Doc)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	userSection(&b)
	return b.String()
}

func renderUnit(project Project, unit Unit, titles map[string]string, usedBy []string) string {
	var b strings.Builder
	frontMatter(&b, unit.Dir)
	fmt.Fprintf(&b, "# %s\n\n", titles[unit.Dir])
	if unit.Doc != "" {
		fmt.Fprintf(&b, "> %s\n\n", unit.Doc)
	}

	fmt.Fprintf(&b, "**Pasta:** `%s`", unit.Dir)
	if group := GroupOf(unit.Dir); group != "" {
		fmt.Fprintf(&b, " · **Módulo:** [[%s]]", HubTitle(group))
	} else {
		fmt.Fprintf(&b, " · **Projeto:** [[%s]]", project.Name)
	}
	b.WriteString("  \n")
	if unit.Language != "" {
		fmt.Fprintf(&b, "**Linguagem:** %s", unit.Language)
		if unit.Package != "" {
			fmt.Fprintf(&b, " · **Pacote:** `%s`", unit.Package)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("## Arquivos\n\n")
	for _, file := range unit.Files {
		fmt.Fprintf(&b, "- `%s` (%s)\n", file.Name, plural(file.Lines, "linha", "linhas"))
	}
	if unit.TestFiles > 0 {
		fmt.Fprintf(&b, "\nTestes: %s.\n", plural(unit.TestFiles, "arquivo", "arquivos"))
	}
	b.WriteString("\n")

	if len(unit.Exported) > 0 {
		b.WriteString("## Símbolos exportados\n\n")
		for i, symbol := range unit.Exported {
			if i == MaxListedSymbols {
				fmt.Fprintf(&b, "- … e mais %d\n", len(unit.Exported)-MaxListedSymbols)
				break
			}
			fmt.Fprintf(&b, "- %s `%s`\n", symbolKind(symbol.Kind), symbol.Name)
		}
		b.WriteString("\n")
	}

	writeLinks(&b, "Depende de", unit.DependsOn, titles)
	writeLinks(&b, "Usado por", usedBy, titles)
	userSection(&b)
	return b.String()
}

func writeLinks(b *strings.Builder, heading string, dirs []string, titles map[string]string) {
	var names []string
	for _, dir := range dirs {
		if title, ok := titles[dir]; ok {
			names = append(names, title)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	fmt.Fprintf(b, "## %s\n\n", heading)
	for _, name := range names {
		fmt.Fprintf(b, "- [[%s]]\n", name)
	}
	b.WriteString("\n")
}

func frontMatter(b *strings.Builder, origin string) {
	fmt.Fprintf(b, "---\n%s\norigem: %s\n---\n\n", GeneratedMarker, origin)
}

func userSection(b *strings.Builder) {
	fmt.Fprintf(b, "%s\n\n%s\n", UserSectionHeading, userSectionPlaceholder)
}

func symbolKind(kind string) string {
	switch kind {
	case "type":
		return "tipo"
	case "func":
		return "função"
	case "method":
		return "método"
	case "const":
		return "constante"
	case "var":
		return "variável"
	}
	return kind
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// IsGenerated reports whether note content was written by the import.
func IsGenerated(content string) bool {
	head, _, _ := strings.Cut(strings.TrimPrefix(content, "\ufeff"), "\n---")
	return strings.HasPrefix(head, "---") && strings.Contains(head, GeneratedMarker)
}

// Merge keeps the user's section of an existing generated note on top of
// freshly generated content.
func Merge(existing, generated string) string {
	at := strings.Index(existing, UserSectionHeading)
	cut := strings.Index(generated, UserSectionHeading)
	if at < 0 || cut < 0 {
		return generated
	}
	return generated[:cut] + existing[at:]
}

// HasUserText reports whether the user wrote anything in a note's section.
func HasUserText(content string) bool {
	at := strings.Index(content, UserSectionHeading)
	if at < 0 {
		return false
	}
	rest := strings.TrimSpace(content[at+len(UserSectionHeading):])
	rest = strings.TrimSpace(strings.Replace(rest, userSectionPlaceholder, "", 1))
	return rest != ""
}
