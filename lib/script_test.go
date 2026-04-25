package lib

import (
	"testing"
)

// makePattern glob -> regex 转换
func TestMakePattern_GlobToRegex(t *testing.T) {
	re := makePattern("气血*/*")
	if re == nil {
		t.Fatal("气血*/* 应生成合法 regex")
	}
	if !re.MatchString("气血】 230     / 230") {
		t.Fatal("气血(.*)/(.*) 应匹配 气血】 230     / 230")
	}
	subs := re.FindStringSubmatch("气血】 230     / 230")
	if len(subs) < 3 {
		t.Fatal("应有 2 个捕获组")
	}
	if subs[1] != "】 230     " {
		t.Fatalf("第1捕获组应为 ] 230     , 实际=[%s]", subs[1])
	}
	if subs[2] != " 230" {
		t.Fatalf("第2捕获组应为  230, 实际=[%s]", subs[2])
	}

}

func TestMakePattern_NamedCapture(t *testing.T) {
	re := makePattern("气血{hp}/*")
	if re == nil {
		t.Fatal("气血{hp}/* 应生成合法 regex")
	}
	if !re.MatchString("气血 100/130") {
		t.Fatal("气血(.*)/(.*) 应匹配 气血 100/130")
	}
	subs := re.FindStringSubmatch("气血 100/130")
	if len(subs) < 3 {
		t.Fatal("应有 2 个捕获组")
	}
	names := re.SubexpNames()
	if len(names) < 2 || names[1] != "hp" {
		t.Fatalf("命名应为 [, hp], 实际=%v", names)
	}
}

func TestMakePattern_PlainText(t *testing.T) {
	re := makePattern("醒来")
	if re != nil {
		t.Fatal("纯文本应返回 nil regex")
	}

}

func TestMakePattern_PureRegex(t *testing.T) {
	re := makePattern(`\s+(\d+)`)
	if re == nil {
		t.Fatal("纯 regex \\s+(\\d+) 应生成合法 regex")
	}
	if !re.MatchString(" 100") {
		t.Fatal("\\s+(\\d+) 应匹配  100")
	}
	subs := re.FindStringSubmatch(" 100")
	if len(subs) < 2 || subs[1] != "100" {
		t.Fatalf("应捕获 100, 实际=%v", subs)
	}

}

func TestMakePattern_MixedNamedAndGlob(t *testing.T) {
	re := makePattern("{a}*{b}?")
	if re == nil {
		t.Fatal("{a}*{b}? 应生成合法 regex")
	}
	if !re.MatchString("xy_z1") {
		t.Fatal("应匹配 xy_z1")
	}
	subs := re.FindStringSubmatch("xy_z1")
	if len(subs) != 5 {
		t.Fatalf("应有 4 个捕获组(full match+4 groups), 实际=%d", len(subs))
	}
	names := re.SubexpNames()
	if len(names) < 5 || names[1] != "a" || names[3] != "b" {
		t.Fatalf("命名应为 [, a, b], 实际=%v", names)
	}
}

// subst 变量替换
func TestSubst_Basic(t *testing.T) {
	VARS["1"] = "100"
	VARS["hp"] = "200"
	s := &Script{}
	tests := []struct {
		input string
		want  string
	}{
		{"say $1", "say 100"},
		{"dazuo $hp", "dazuo 200"},
		{"$$hello", "$hello"},
		{"no vars", "no vars"},
		{"$unknown", "$unknown"},
		{"$1 $hp", "100 200"},
		{"$hp$1", "200100"},
	}
	for _, tt := range tests {
		got := s.subst(tt.input)
		if got != tt.want {
			t.Errorf("subst(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSubst_Arithmetic(t *testing.T) {
	VARS["1"] = "100"
	VARS["hp"] = "200"
	s := &Script{}
	tests := []struct {
		input string
		want  string
	}{
		{"$1-20", "80"},
		{"$hp+50", "250"},
		{"$1*2", "200"},
		{"$hp/5", "40"},
		{"$hp*0.8", "160"},
		{"$1+10.5", "110"},
	}
	for _, tt := range tests {
		got := s.subst(tt.input)
		if got != tt.want {
			t.Errorf("subst(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSubst_NonNumericNoArithmetic(t *testing.T) {
	VARS["hp"] = "abc"
	s := &Script{}
	got := s.subst("$hp-20")
	if got != "abc-20" {
		t.Errorf("非数字变量不应触发算术, got=%q", got)
	}
}

func TestSubst_MissingVarNoArithmetic(t *testing.T) {
	clear(VARS)
	s := &Script{}
	got := s.subst("dazuo $hp-20")
	if got != "dazuo $hp-20" {
		t.Errorf("缺失变量不应触发算术, got=%q", got)
	}
}

// waitKeyword 集成测试
func TestWaitKeyword_GlobCapture(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	// 启动 waitKeyword 协程
	done := make(chan bool)
	go func() {
		done <- s.waitKeyword("气血*/*")
	}()

	// 投喂匹配文本
	s.waitCh <- "气血】 230     / 230"

	// 等待结果
	if !<-done {
		t.Fatal("waitKeyword 应返回 true")
	}
	if VARS["1"] != "】 230" {
		t.Fatalf("vars[1] 应为 ] 230 , 实际=[%s]", VARS["1"])
	}
	if VARS["2"] != "230" {
		t.Fatalf("vars[2] 应为 230, 实际=[%s]", VARS["2"])
	}
}

func TestWaitKeyword_NamedCapture(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	done := make(chan bool)
	go func() {
		done <- s.waitKeyword("气血{hp}/*")
	}()

	s.waitCh <- "气血 100/130"

	if !<-done {
		t.Fatal("waitKeyword 应返回 true")
	}
	if VARS["hp"] != "100" {
		t.Fatalf("vars[hp] 应为 100, 实际=[%s]", VARS["hp"])
	}
}

func TestWaitKeyword_PlainText(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	done := make(chan bool)
	go func() {
		done <- s.waitKeyword("醒来")
	}()

	s.waitCh <- "你睡了一觉,醒来"

	if !<-done {
		t.Fatal("waitKeyword 纯文本应返回 true")
	}
}

func TestWaitKeyword_Regex(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	done := make(chan bool)
	go func() {
		done <- s.waitKeyword(`\s+(\d+)`)
	}()

	s.waitCh <- "气血 100/130"

	if !<-done {
		t.Fatal("waitKeyword regex 应返回 true")
	}
	if VARS["1"] != "100" {
		t.Fatalf("vars[1] 应为 100, 实际=[%s]", VARS["1"])
	}
}

// Script.Run 完整集成测试
func TestRun_GlobCaptureAndSubst(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	// 在另一个协程中运行
	go s.Run("hp:气血*/*;say $1")

	// 读取发送到 wc 的命令
	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}

	// 投喂匹配文本
	s.waitCh <- "气血】 230     / 230"

	// 读取替换后的命令
	cmd2 := <-wc
	if cmd2 != "say 】 230" {
		t.Fatalf("第2条命令应为 say ] 230, 实际=[%s]", cmd2)
	}
}

func TestRun_NamedCaptureAndSubst(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	go s.Run("hp:气血{hp}/*;dazuo $hp")

	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}

	s.waitCh <- "气血 100/130"

	cmd2 := <-wc
	if cmd2 != "dazuo 100" {
		t.Fatalf("第2条命令应为 dazuo 100, 实际=[%s]", cmd2)
	}
}

func TestRun_ArithmeticSubst(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	go s.Run("hp:气血{hp}/*;dazuo $hp-20")

	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}

	s.waitCh <- "气血 100/130"

	cmd2 := <-wc
	if cmd2 != "dazuo 80" {
		t.Fatalf("第2条命令应为 dazuo 80, 实际=[%s]", cmd2)
	}
}

func TestRun_PlainTextBackwardCompat(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	go s.Run("sleep:醒来;drink")

	cmd1 := <-wc
	if cmd1 != "sleep" {
		t.Fatalf("第1条命令应为 sleep, 实际=[%s]", cmd1)
	}

	s.waitCh <- "你睡了一觉,终于醒来"

	cmd2 := <-wc
	if cmd2 != "drink" {
		t.Fatalf("第2条命令应为 drink, 实际=[%s]", cmd2)
	}
}

// regexp.QuoteMeta 行为验证 — 确保 / 不被转义
func TestMakePattern_SlashNotEscaped(t *testing.T) {
	re := makePattern("a*/b*")
	if re == nil {
		t.Fatal("a*/b* 应生成合法 regex")
	}
	if !re.MatchString("axx/byy") {
		t.Fatal("a(.*)/b(.*) 应匹配 axx/byy")
	}
	subs := re.FindStringSubmatch("axx/byy")
	if len(subs) != 3 {
		t.Fatalf("应有 2 个捕获组, 实际=%d", len(subs))
	}
}

// 确保 containsRegexMeta 仍可用（内部依赖）
func TestContainsRegexMeta(t *testing.T) {
	if !containsRegexMeta("a|b") {
		t.Error("应检测到 |")
	}
	if !containsRegexMeta("(ab)") {
		t.Error("应检测到 ()")
	}
	if !containsRegexMeta("a*b") {
		t.Error("应检测到 *")
	}
	if !containsRegexMeta("a+b") {
		t.Error("应检测到 +")
	}
	if containsRegexMeta("醒来") {
		t.Error("纯文本不应触发")
	}
}

func TestRun_GlobCaptureMultipleSlash(t *testing.T) {
	wc := make(chan string, 10)
	s := NewScript(wc, nil)

	go s.Run("hp:气血】*/*;say $1")

	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}

	s.waitCh <- "│【气血】 230     / 230      [100%]    │【内力】 486     / 251     (+   0)  │"

	cmd2 := <-wc
	if cmd2 != "say 230" {
		t.Fatalf("$1 应捕获 230 而不是冗长文本, 实际=[%s]", cmd2)
	}
}


func TestMakePattern_MixedGlobAndNamedCapture(t *testing.T) {
	re := makePattern("【气血】{hp}/*[*【内力】{nl}/* (")
	if re == nil {
		t.Fatal("应生成合法 regex")
	}
	subs := re.FindStringSubmatch("│【气血】 230     / 230      [100%]    │【内力】 502     / 251     (+   0)  │")
	if len(subs) < 6 {
		t.Fatalf("应有 5 个捕获组, 实际=%d", len(subs)-1)
	}
	if subs[1] != " 230     " {
		t.Fatalf("组1(hp) 应为 230 , 实际=[%s]", subs[1])
	}
	if subs[4] != " 502     " {
		t.Fatalf("组4(nl) 应为 502 , 实际=[%s]", subs[4])
	}
	names := re.SubexpNames()
	if names[1] != "hp" || names[4] != "nl" {
		t.Fatalf("命名映射错误: hp→组%d, nl→组%d", func() int {
			for i, n := range names {
				if n == "hp" { return i }
			}
			return -1
		}(), func() int {
			for i, n := range names {
				if n == "nl" { return i }
			}
			return -1
		}())
	}
}


func TestMakePattern_NamedCaptureAtEnd(t *testing.T) {
	re := makePattern("#*#*,*,{hp},*,*,{js}")
	if re == nil {
		t.Fatal("应生成合法 regex")
	}
	text := "#5988,16739,359,718,458,916\n#303,303,303,243,243,243\n> "
	if !re.MatchString(text) {
		t.Fatal("应匹配多行 hpbrief 输出")
	}
	subs := re.FindStringSubmatch(text)
	if len(subs) < 8 {
		t.Fatalf("应有 7 个捕获组, 实际=%d", len(subs)-1)
	}
	if subs[4] != "303" {
		t.Fatalf("hp 应为 303, 实际=[%s]", subs[4])
	}
	if subs[7] != "243\n> " {
		t.Fatalf("js 应为 243\\n> , 实际=[%s]", subs[7])
	}
	names := re.SubexpNames()
	if names[4] != "hp" || names[7] != "js" {
		t.Fatalf("命名映射错误: hp→组4, js→组7, 实际=hp→组%d, js→组%d",
			func() int { for i, n := range names { if n == "hp" { return i } }; return -1 }(),
			func() int { for i, n := range names { if n == "js" { return i } }; return -1 }())
	}
}
func TestMakePattern_MultilineGlob(t *testing.T) {
	re := makePattern("#*#*")
	if re == nil {
		t.Fatal("#*#* 应生成合法 regex")
	}
	text := "#26018,25372,252,362,121,141\n#230,230,230,130,130,130\n#0,80,320,376,0,0\n> "
	if !re.MatchString(text) {
		t.Fatal("(?s)#(.*?)#(.*) 应跨行匹配多行文本")
	}
	subs := re.FindStringSubmatch(text)
	if len(subs) < 3 {
		t.Fatal("应有 2 个捕获组")
	}
	if subs[1] != "26018,25372,252,362,121,141\n" {
		t.Fatalf("组1应捕获两\x23间内容含换行, 实际=[%s]", subs[1])
	}
}

// 错误处理: 不闭合的 { 应被安全处理
func TestMakePattern_UnclosedBrace(t *testing.T) {
	re := makePattern("a{b")
	if re == nil {
		t.Fatal("不闭合 { 也应生成 regex")
	}
	if !re.MatchString("a{b") {
		t.Fatal("a{b → 应字面匹配 a{b")
	}

}

// aliases: 普通命令别名展开
func TestRun_AliasBasic(t *testing.T) {
	wc := make(chan string, 10)
	aliases := map[string]string{"chihe": "hp:气血{hp}/*;drink"}
	s := NewScript(wc, aliases)

	go s.Run("chihe;say done")

	// chihe → hp:气血{hp}/*
	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}
	s.waitCh <- "气血 100/130"
	if VARS["hp"] != "100" {
		t.Fatalf("vars[hp] 应为 100, 实际=[%s]", VARS["hp"])
	}

	// chihe → drink
	cmd2 := <-wc
	if cmd2 != "drink" {
		t.Fatalf("第2条命令应为 drink, 实际=[%s]", cmd2)
	}

	// say done
	cmd3 := <-wc
	if cmd3 != "say done" {
		t.Fatalf("第3条命令应为 say done, 实际=[%s]", cmd3)
	}
}

// aliases: cmd:keyword 别名展开
func TestRun_AliasCmdKeyword(t *testing.T) {
	wc := make(chan string, 10)
	aliases := map[string]string{"chihe": "hp:气血{hp}/*;drink"}
	s := NewScript(wc, aliases)

	go s.Run("chihe:醒来;say done")

	// chihe → hp:气血{hp}/*
	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}
	s.waitCh <- "气血 100/130"

	// chihe → drink
	cmd2 := <-wc
	if cmd2 != "drink" {
		t.Fatalf("第2条命令应为 drink, 实际=[%s]", cmd2)
	}

	// 然后等待关键字 "醒来"
	s.waitCh <- "你睡了一觉,终于醒来"

	// say done
	cmd3 := <-wc
	if cmd3 != "say done" {
		t.Fatalf("第3条命令应为 say done, 实际=[%s]", cmd3)
	}
}

// aliases: 别名展开 + #jmp 索引不变
func TestRun_AliasJmpPreserved(t *testing.T) {
	wc := make(chan string, 10)
	aliases := map[string]string{"chihe": "hp:气血{hp}/*;drink"}
	s := NewScript(wc, aliases)

	go s.Run("chihe;dazuo 100;#jmp2")

	// i=0: chihe → hp:气血{hp}/*
	cmd1 := <-wc
	if cmd1 != "hp" {
		t.Fatalf("第1条命令应为 hp, 实际=[%s]", cmd1)
	}
	s.waitCh <- "气血 100/130"

	// chihe → drink
	cmd2 := <-wc
	if cmd2 != "drink" {
		t.Fatalf("第2条命令应为 drink, 实际=[%s]", cmd2)
	}

	// i=1: dazuo 100
	cmd3 := <-wc
	if cmd3 != "dazuo 100" {
		t.Fatalf("dazuo 应为 dazuo 100, 实际=[%s]", cmd3)
	}

	// #jmp2 应跳回 dazuo 100（原第2条，不是 drink）
	cmd4 := <-wc
	if cmd4 != "dazuo 100" {
		t.Fatalf("#jmp2 应跳到 dazuo 100, 实际=[%s]", cmd4)
	}
}

// aliases: %N 别名展开
func TestRun_AliasPercentN(t *testing.T) {
	wc := make(chan string, 10)
	aliases := map[string]string{"chihe": "drink"}
	s := NewScript(wc, aliases)

	// %100 = 必然执行
	go s.Run("%100 chihe;say done")

	cmd1 := <-wc
	if cmd1 != "drink" {
		t.Fatalf("别名展开应为 drink, 实际=[%s]", cmd1)
	}
	cmd2 := <-wc
	if cmd2 != "say done" {
		t.Fatalf("应为 say done, 实际=[%s]", cmd2)
	}
}

// aliases: #N 别名展开
func TestRun_AliasRepeatN(t *testing.T) {
	wc := make(chan string, 10)
	aliases := map[string]string{"hello": "say hi"}
	s := NewScript(wc, aliases)

	go s.Run("#2 hello;say done")

	cmd1 := <-wc
	if cmd1 != "say hi" {
		t.Fatalf("第1次应为 say hi, 实际=[%s]", cmd1)
	}
	cmd2 := <-wc
	if cmd2 != "say hi" {
		t.Fatalf("第2次应为 say hi, 实际=[%s]", cmd2)
	}
	cmd3 := <-wc
	if cmd3 != "say done" {
		t.Fatalf("应为 say done, 实际=[%s]", cmd3)
	}
}

// evalCompare 比较运算符
func TestEvalCompare(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		{"5>3", true},
		{"3>5", false},
		{"3<5", true},
		{"5<3", false},
		{"5>=5", true},
		{"5>=6", false},
		{"5<=5", true},
		{"6<=5", false},
		{"5=5", true},
		{"5=6", false},
		{"5!=6", true},
		{"5!=5", false},
		{"100>100", false},
		{"abc>5", false},   // 非法左值 → 0>5
		{"5>xyz", true},    // 非法右值 → 5>0
		{"abc>xyz", false}, // 全非法 → 0>0
	}
	for _, tt := range tests {
		got := evalCompare(tt.expr)
		if got != tt.want {
			t.Errorf("evalCompare(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

// #if 条件真 → 跳转
func TestRun_IfJump(t *testing.T) {
	wc := make(chan string, 10)
	VARS["nl"] = "150"
	defer delete(VARS, "nl")
	s := NewScript(wc, nil)

	go s.Run("dazuo;#if $nl>100 1;sleep")

	cmd1 := <-wc
	if cmd1 != "dazuo" {
		t.Fatalf("第1条应为 dazuo, 实际=[%s]", cmd1)
	}
	// #if $nl>100 1 → 150>100 → true → 跳回命令 1
	cmd2 := <-wc
	if cmd2 != "dazuo" {
		t.Fatalf("跳转后应为 dazuo, 实际=[%s]", cmd2)
	}
}

// #if 条件假 → fallthrough
func TestRun_IfFallthrough(t *testing.T) {
	wc := make(chan string, 10)
	VARS["nl"] = "50"
	defer delete(VARS, "nl")
	s := NewScript(wc, nil)

	go s.Run("dazuo;#if $nl>100 1;sleep")

	cmd1 := <-wc
	if cmd1 != "dazuo" {
		t.Fatalf("第1条应为 dazuo, 实际=[%s]", cmd1)
	}
	// #if $nl>100 1 → 50>100 → false → fallthrough
	cmd2 := <-wc
	if cmd2 != "sleep" {
		t.Fatalf("fallthrough 后应为 sleep, 实际=[%s]", cmd2)
	}
}

// #if 条件真 → 执行命令（单命令）
func TestRun_IfExecCmd(t *testing.T) {
	wc := make(chan string, 10)
	VARS["nl"] = "150"
	defer delete(VARS, "nl")
	s := NewScript(wc, nil)

	go s.Run("dazuo;#if $nl>100 drink;sleep")

	cmd1 := <-wc
	if cmd1 != "dazuo" {
		t.Fatalf("第1条应为 dazuo, 实际=[%s]", cmd1)
	}
	// #if $nl>100 drink → 150>100 → true → executeCmd("drink")
	cmd2 := <-wc
	if cmd2 != "drink" {
		t.Fatalf("条件真应执行 drink, 实际=[%s]", cmd2)
	}
	// sleep 是 #if 后的下一条（#if 不跳转，继续往下）
	cmd3 := <-wc
	if cmd3 != "sleep" {
		t.Fatalf("ifeq 后应为 sleep, 实际=[%s]", cmd3)
	}
}

// #if 条件真 → 执行多词命令
func TestRun_IfExecMultiWord(t *testing.T) {
	wc := make(chan string, 10)
	VARS["js"] = "200"
	defer delete(VARS, "js")
	s := NewScript(wc, nil)

	go s.Run("#if $js>100 tuna 100;sleep")

	// #if $js>100 tuna 100 → 200>100 → true → executeCmd("tuna 100")
	cmd1 := <-wc
	if cmd1 != "tuna 100" {
		t.Fatalf("多词命令应为 tuna 100, 实际=[%s]", cmd1)
	}
	cmd2 := <-wc
	if cmd2 != "sleep" {
		t.Fatalf("#if 后应为 sleep, 实际=[%s]", cmd2)
	}
}

// #if 执行带关键字的命令
func TestRun_IfExecCmdWithKeyword(t *testing.T) {
	wc := make(chan string, 10)
	VARS["nl"] = "150"
	defer delete(VARS, "nl")
	s := NewScript(wc, nil)

	go s.Run("#if $nl>100 drink:喝了;sleep")

	// #if → executeCmd("drink:喝了")
	cmd1 := <-wc
	if cmd1 != "drink" {
		t.Fatalf("命令应为 drink, 实际=[%s]", cmd1)
	}
	// 等待关键字
	s.waitCh <- "咕噜咕噜喝了一大口"
	// 然后 sleep
	cmd2 := <-wc
	if cmd2 != "sleep" {
		t.Fatalf("#if 后应为 sleep, 实际=[%s]", cmd2)
	}
}

// ExpandAlias 测试
func TestExpandAlias(t *testing.T) {
	tests := []struct {
		name    string
		aliases map[string]string
		input   string
		want    string
		wantOk  bool
	}{
		{"无参别名", map[string]string{"chi": "drink"}, "chi", "drink", true},
		{"单参数", map[string]string{"chi": "eat $A1"}, "chi jitui", "eat jitui", true},
		{"多参数", map[string]string{"ci": "eat $A1 $A2"}, "ci jitui ya", "eat jitui ya", true},
		{"参数不足", map[string]string{"chi": "eat $A1 and $A2"}, "chi jitui", "eat jitui and ", true},
		{"别名不存在", map[string]string{}, "unknown", "unknown", false},
		{"保留 $name 变量", map[string]string{"chi": "eat $A1 $hp"}, "chi jitui", "eat jitui $hp", true},
		{"多参无参", map[string]string{"chi": "drink"}, "chi jitui", "drink", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExpandAlias(tt.aliases, tt.input)
			if ok != tt.wantOk {
				t.Errorf("ExpandAlias() ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("ExpandAlias() = %q, want %q", got, tt.want)
			}
		})
	}
}
