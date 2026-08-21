package bxn_point_cloud_renew

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestShouldParseFrameFilterIDsWhenQuotedWithChineseComma(t *testing.T) {
	got := parseFrameFilterIDs(`"id1，id2, 'id3'"`)
	want := []string{"id1", "id2", "id3"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestShouldReturnNoFilterWhenFrameFilterBlank(t *testing.T) {
	for _, spec := range []string{"", "  ", `""`} {
		if got := parseFrameFilterIDs(spec); got != nil {
			t.Fatalf("spec %q: expected nil, got %v", spec, got)
		}
	}
}

func TestShouldNormalizeRecordWhenExtraColumnsProvided(t *testing.T) {
	rec := normalizeRecord([]string{" D:\\old ", "D:\\new", " 60410257 ", "id1", "id2"})
	if rec == nil {
		t.Fatalf("expected record, got nil")
	}
	if rec[0] != "D:\\old" || rec[1] != "D:\\new" || rec[2] != "60410257" {
		t.Fatalf("unexpected record: %v", rec)
	}
	if rec[3] != "id1,id2" {
		t.Fatalf("expected extra columns merged to id1,id2, got %q", rec[3])
	}
}

func TestShouldDropRecordWhenRequiredColumnsMissing(t *testing.T) {
	if normalizeRecord([]string{"D:\\old"}) != nil {
		t.Fatalf("expected nil when only one column")
	}
	if normalizeRecord([]string{"", "D:\\new", "60410257"}) != nil {
		t.Fatalf("expected nil when old dir empty")
	}
}

func TestShouldDetectHeaderWhenFirstRowLooksLikeHeader(t *testing.T) {
	if !looksLikeHeader([]string{"标题", "", "", ""}) {
		t.Fatalf("expected 标题 row to be header")
	}
	if !looksLikeHeader([]string{"老包路径", "新项目路径", "项目ID"}) {
		t.Fatalf("expected keyword header row to be header")
	}
}

func TestShouldKeepFirstRowWhenItIsData(t *testing.T) {
	if looksLikeHeader([]string{`D:\old\pkg_00028`, `D:\new\proj`, "60410257"}) {
		t.Fatalf("expected path-like first row to be data")
	}
}

func TestShouldPackageToUnixSecWhenValidPackageName(t *testing.T) {
	sec, err := packageToUnixSec("2026-03-04-14-15-20_out_source_00028")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 3, 4, 14, 15, 20, 0, chinaLoc()).Unix()
	if sec != want {
		t.Fatalf("expected %d, got %d", want, sec)
	}
}

func TestShouldErrorWhenPackageNameHasNoTimestamp(t *testing.T) {
	if _, err := packageToUnixSec("out_source_00028"); err == nil {
		t.Fatalf("expected error for package name without timestamp")
	}
}

func TestShouldMatchByFramesWhenMixedPoints(t *testing.T) {
	frame := taskFrame{
		id: "f1",
		polygon: [][]float64{
			{113.00, 23.00}, {113.01, 23.00}, {113.01, 23.01}, {113.00, 23.01},
		},
	}
	oldPts := []trajPoint{
		{trajLine: trajLine{ts: "100"}, lon: 113.005, lat: 23.005}, // 框内，新老共有 → 替换
		{trajLine: trajLine{ts: "200"}, lon: 113.005, lat: 23.005}, // 框内，老包独有 → 删除
		{trajLine: trajLine{ts: "300"}, lon: 114.00, lat: 24.00},   // 框外 → 保留
	}
	newPts := []trajPoint{
		{trajLine: trajLine{ts: "100"}, lon: 113.005, lat: 23.005}, // 替换源
		{trajLine: trajLine{ts: "400"}, lon: 113.006, lat: 23.006}, // 新包独有 → 跳过
	}

	res := matchByFrames(oldPts, newPts, []taskFrame{frame})

	if res.stat.Replaced != 1 || res.stat.Deleted != 1 || res.stat.Skipped != 1 {
		t.Fatalf("unexpected stat: %+v", res.stat)
	}
	if !res.replaceTS["100"] || !res.keepTS["100"] || !res.keepTS["300"] {
		t.Fatalf("unexpected keep/replace sets: %+v", res)
	}
	if len(res.keptTS) != 1 || res.keptTS[0] != "300" {
		t.Fatalf("expected keptTS=[300], got %v", res.keptTS)
	}
}

func TestShouldFilterFramesByIDWhenAllPresent(t *testing.T) {
	frames := []taskFrame{{id: "a"}, {id: "b"}}
	got, err := filterFramesByID(frames, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].id != "a" {
		t.Fatalf("expected only frame a, got %+v", got)
	}
}

func TestShouldErrorWhenFrameIDMissingInQueryResult(t *testing.T) {
	frames := []taskFrame{{id: "a"}, {id: "b"}}
	_, err := filterFramesByID(frames, []string{"a", "c"})
	if err == nil {
		t.Fatalf("expected error for missing frame id")
	}
	if !strings.Contains(err.Error(), "c") {
		t.Fatalf("expected error to mention missing id c, got %v", err)
	}
}

func TestShouldParseCLIArgsWhenBatchMode(t *testing.T) {
	var out bytes.Buffer
	a, err := parseCLIArgs([]string{"-input", "in.csv", "-output", "out", "-resume"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.inputPath != "in.csv" || a.outputDir != "out" || !a.resume {
		t.Fatalf("unexpected args: %+v", a)
	}
}

func TestShouldParseCLIArgsWhenSingleMode(t *testing.T) {
	var out bytes.Buffer
	a, err := parseCLIArgs([]string{
		"-old", `D:\old_pkg`, "-new", `D:\new_proj`,
		"-frame-project-id", "60410257", "-output", "out",
	}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.oldDir == "" || a.newDir == "" || a.frameProjID != "60410257" {
		t.Fatalf("unexpected args: %+v", a)
	}
}

func TestShouldErrorWhenOldNewAndInputBothGiven(t *testing.T) {
	var out bytes.Buffer
	_, err := parseCLIArgs([]string{
		"-input", "in.csv", "-old", "a", "-new", "b",
		"-frame-project-id", "1", "-output", "o",
	}, &out)
	if err == nil {
		t.Fatalf("expected mutual exclusion error")
	}
}

func TestShouldErrorWhenOutputMissing(t *testing.T) {
	var out bytes.Buffer
	if _, err := parseCLIArgs([]string{"-input", "in.csv"}, &out); err == nil {
		t.Fatalf("expected missing output error")
	}
}

func TestShouldErrorWhenSingleModeMissingFrameProjectID(t *testing.T) {
	var out bytes.Buffer
	_, err := parseCLIArgs([]string{"-old", "a", "-new", "b", "-output", "o"}, &out)
	if err == nil {
		t.Fatalf("expected missing frame-project-id error")
	}
}

func TestShouldReadCsvInputWhenFileHasHeaderAndGBK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.csv")

	// GBK 编码 + 表头行，验证编码识别与表头跳过
	content := "老包路径,新项目路径,项目ID,框ID\n" +
		`D:\old1,D:\new1,60410257,id1` + "\n" +
		`D:\old2,D:\new2,60410257,"idA,idB"` + "\n"
	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(content))
	if err != nil {
		t.Fatalf("encode GBK failed: %v", err)
	}
	if err := os.WriteFile(path, gbk, 0644); err != nil {
		t.Fatalf("write input failed: %v", err)
	}

	var out bytes.Buffer
	records, err := readInputFile(path, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0][0] != `D:\old1` || records[0][2] != "60410257" || records[0][3] != "id1" {
		t.Fatalf("unexpected first record: %v", records[0])
	}
	if records[1][3] != "idA,idB" {
		t.Fatalf("expected quoted frame ids idA,idB, got %q", records[1][3])
	}
	if !strings.Contains(out.String(), "header") {
		t.Fatalf("expected header skip notice in output: %q", out.String())
	}
}
