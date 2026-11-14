//go:build race

package ui

// raceEnabled 为 true 表示测试在 -race 下运行；竞态检测会让每帧慢好几倍，依赖耗时的测试要跳过。
const raceEnabled = true
