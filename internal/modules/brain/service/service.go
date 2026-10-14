// Package service holds the project import use case: read the structure of
// the code in the open vault and keep a set of linked notes about it in
// model.OutputDir.
package service

import (
	"context"
	"strings"

	"parallelepiped/internal/modules/brain/model"
	errs "parallelepiped/internal/platform/errors"
	"parallelepiped/internal/platform/validation"
)

// Analyzer reads the project; repository.Analyzer implements it.
type Analyzer interface {
	Analyze(ctx context.Context, root string) (model.Project, error)
	// VaultNotes returns the lower-case titles of notes outside
	// model.OutputDir and the paths of the notes inside it.
	VaultNotes(ctx context.Context, root string) (map[string]bool, []string, error)
}

// RootProvider returns the folder of the open vault.
type RootProvider interface {
	Root() (string, error)
}

// Notes reads and writes vault notes. Writes must go through the notes
// module so the index and watcher see them as the app's own; the adapter
// lives in the composition root and speaks plain strings.
type Notes interface {
	Read(ctx context.Context, path string) (content string, found bool, err error)
	Save(ctx context.Context, path, content string) error
	Delete(ctx context.Context, path string) error
}

// Progress is told how many notes have been processed so far.
type Progress interface {
	ImportProgress(done, total int)
}

// ImportResult summarises one import.
type ImportResult struct {
	// IndexNote is the path of the project's overview note.
	IndexNote string
	Units     int
	Created   int
	Updated   int
	Unchanged int
	Removed   int
	// Skipped are paths taken by notes the import did not write; they are
	// never overwritten.
	Skipped []string
	// Kept are generated notes whose code is gone but that hold the user's
	// own text, so they were not removed.
	Kept []string
}

type Service struct {
	analyzer Analyzer
	roots    RootProvider
	notes    Notes
	progress Progress
}

func New(analyzer Analyzer, roots RootProvider, notes Notes, progress Progress) *Service {
	return &Service{analyzer: analyzer, roots: roots, notes: notes, progress: progress}
}

// Import maps the code of the open vault into notes. It is safe to run again:
// unchanged notes are left alone, the user's section of each note is kept
// and notes not written by the import are never touched.
func (s *Service) Import(ctx context.Context) (ImportResult, error) {
	root, err := s.roots.Root()
	if err != nil {
		return ImportResult{}, err
	}
	project, err := s.analyzer.Analyze(ctx, root)
	if err != nil {
		return ImportResult{}, errs.PreserveOr(errs.Internal, "analyze project", err)
	}
	if len(project.Units) == 0 {
		return ImportResult{}, validation.Field("project", "Nenhuma pasta com código-fonte foi encontrada neste cofre.")
	}
	existingTitles, previous, err := s.analyzer.VaultNotes(ctx, root)
	if err != nil {
		return ImportResult{}, errs.PreserveOr(errs.Internal, "list vault notes", err)
	}

	notes := model.RenderNotes(project, existingTitles)
	result := ImportResult{IndexNote: notes[0].Path, Units: len(project.Units)}
	total := len(notes) + len(previous)
	written := make(map[string]bool, len(notes))

	for i, note := range notes {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		written[strings.ToLower(note.Path)] = true
		if err := s.write(ctx, note, &result); err != nil {
			return result, err
		}
		s.progress.ImportProgress(i+1, total)
	}

	for i, path := range previous {
		if !written[strings.ToLower(path)] {
			if err := s.retire(ctx, path, &result); err != nil {
				return result, err
			}
		}
		s.progress.ImportProgress(len(notes)+i+1, total)
	}
	return result, nil
}

func (s *Service) write(ctx context.Context, note model.NoteFile, result *ImportResult) error {
	existing, found, err := s.notes.Read(ctx, note.Path)
	if err != nil {
		return errs.PreserveOr(errs.Internal, "read note", err)
	}
	content := note.Content
	switch {
	case !found:
		result.Created++
	case !model.IsGenerated(existing):
		result.Skipped = append(result.Skipped, note.Path)
		return nil
	default:
		content = model.Merge(existing, note.Content)
		if content == existing {
			result.Unchanged++
			return nil
		}
		result.Updated++
	}
	return errs.PreserveOr(errs.Internal, "write note", s.notes.Save(ctx, note.Path, content))
}

// retire removes a generated note whose code no longer exists, unless the
// user wrote in it or it was not generated at all.
func (s *Service) retire(ctx context.Context, path string, result *ImportResult) error {
	content, found, err := s.notes.Read(ctx, path)
	if err != nil {
		return errs.PreserveOr(errs.Internal, "read note", err)
	}
	if !found || !model.IsGenerated(content) {
		return nil
	}
	if model.HasUserText(content) {
		result.Kept = append(result.Kept, path)
		return nil
	}
	if err := s.notes.Delete(ctx, path); err != nil {
		return errs.PreserveOr(errs.Internal, "remove note", err)
	}
	result.Removed++
	return nil
}
