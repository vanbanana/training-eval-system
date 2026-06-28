package service

import (
	"strings"
	"testing"
)

func TestIsGarbledLine(t *testing.T) {
	garbled := []string{
		"涶粪惰祫䆱䶟㒘嫉坖",         // Ext-A + rare CJK
		"琀敨敭琯敨敭琯敨敭慍慮敧",      // byte-swapped ASCII ("the m...")
		"桴浥䵥湡条牥砮汭䭐",         // byte-swapped ASCII ("tmeManager")
		"卋偏潲畤瑣畂汩噤牥",         // byte-swapped ASCII (KSOProductBuildVer)
		"襃辿䫊焇铵锩士徙",          // Ext-A present
		"妓$撲$嬡(愃)楞)㗜*穇*䌾+偎", // dense Ext-A interspersed with ASCII punctuation
		"一蝓䱥瞈i㬀逎蘁",          // Ext-A with a stray ASCII letter
	}
	for _, g := range garbled {
		if !isGarbledLine(g) {
			t.Errorf("expected garbled line to be detected: %q", g)
		}
	}

	real := []string{
		"广东科学技术职业学院",
		"计算机工程技术学院（人工智能学院）",
		"四、心得体会（在学习过程中遇到的困难）",
		"我通过本次实训学会了使用各种Linux命令操作文件和目录",
		"感谢老师的耐心指导让我对操作系统有了更深的理解",
		"这次实验让我明白了实践的重要性理论结合实际才能真正掌握知识",
		"任务 1题目Linux命令基础及pwd,su,man or --help,cd,ls",
		"1）在桌面打开终端，查看当前目录",
	}
	for _, r := range real {
		if isGarbledLine(r) {
			t.Errorf("real Chinese line wrongly flagged as garbled: %q", r)
		}
	}
}

func TestCleanTextNormalizesCR(t *testing.T) {
	in := "标题一\r正文第一行\r\n正文第二行"
	out := CleanText(in)
	if strings.Contains(out, "\r") {
		t.Fatalf("CleanText should normalize CR, got %q", out)
	}
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("expected 2 newlines after normalization, got %q", out)
	}
}

func TestAnalyzeReadabilityStripsGarbledKeepsReadable(t *testing.T) {
	raw := strings.Join([]string{
		"广东科学技术职业学院",
		"四、心得体会（在学习过程中遇到的困难）",
		"通过本次实训我掌握了基本的命令操作",
		"涶粪惰祫䆱䶟㒘嫉坖",
		"琀敨敭琯敨敭琯敨敭慍慮敧",
		"桴浥䵥湡条牥砮汭䭐",
	}, "\r")

	res := AnalyzeReadability(raw)
	if !res.IsReadable {
		t.Fatalf("document with readable body should stay readable")
	}
	if strings.Contains(res.CleanText, "涶粪") || strings.Contains(res.CleanText, "琀敨敭") {
		t.Fatalf("garbled lines should be removed, got: %q", res.CleanText)
	}
	if !strings.Contains(res.CleanText, "通过本次实训") {
		t.Fatalf("real content must be preserved, got: %q", res.CleanText)
	}
	found := false
	for _, w := range res.Warnings {
		if w == "garbled_segments_removed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected garbled_segments_removed warning, got %v", res.Warnings)
	}
}
