package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupsKeepTheLast15(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "progress.json"), `{"version":2}`)
	for i := 0; i < 20; i++ {
		if _, err := BackupSaves(dir, "update"); err != nil {
			t.Fatal(err)
		}
	}
	bs := Backups(dir)
	if len(bs) != 15 {
		t.Fatalf("%d backups kept, want 15", len(bs))
	}
	if readFile(t, filepath.Join(bs[0].Path, "progress.json")) != `{"version":2}` || bs[0].Reason == "" {
		t.Errorf("newest backup %+v", bs[0])
	}
	if p, _ := BackupSaves(t.TempDir(), "x"); p != "" {
		t.Error("an empty folder made a backup")
	}
}

func TestRestoreBacksUpTheCurrentSavesFirst(t *testing.T) {
	dir := t.TempDir()
	save := filepath.Join(dir, "progress.json")
	writeFile(t, save, `{"version":2,"n":1}`)
	b, _ := BackupSaves(dir, "update")
	writeFile(t, save, `{"version":2,"n":2}`)
	if err := RestoreBackup(dir, filepath.Base(b)); err != nil {
		t.Fatal(err)
	}
	if readFile(t, save) != `{"version":2,"n":1}` {
		t.Error("the backup was not restored")
	}
	found := false
	for _, x := range Backups(dir) {
		if x.Reason == "before-restore" && readFile(t, filepath.Join(x.Path, "progress.json")) == `{"version":2,"n":2}` {
			found = true
		}
	}
	if !found {
		t.Error("the saves being replaced were not backed up")
	}
}

func TestExportThenImport(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(a, "progress.json"), `{"version":2,"who":"a"}`)
	writeFile(t, filepath.Join(a, "config.json"), `{"version":4}`)
	zip := filepath.Join(t.TempDir(), "saves.zip")
	if err := ExportSaves(a, zip); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(b, "progress.json"), `{"version":2,"who":"b"}`)
	if err := ImportSaves(b, zip); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(b, "progress.json")) != `{"version":2,"who":"a"}` {
		t.Error("import did not bring the progress in")
	}
	if len(Backups(b)) != 1 {
		t.Error("import did not back up first")
	}

	bad := filepath.Join(t.TempDir(), "bad.zip")
	writeFile(t, filepath.Join(a, "progress.json"), `not json`)
	ExportSaves(a, bad)
	if err := ImportSaves(b, bad); err == nil {
		t.Error("a damaged export was imported")
	}
	if readFile(t, filepath.Join(b, "progress.json")) != `{"version":2,"who":"a"}` {
		t.Error("a refused import changed the saves")
	}
	if err := ExportSaves(t.TempDir(), filepath.Join(t.TempDir(), "none.zip")); err == nil {
		t.Error("exported nothing without saying so")
	}
	if _, err := os.Stat(filepath.Join(b, "progress.json.importing")); err == nil {
		t.Error("import left a temp file")
	}
}
