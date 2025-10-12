package main

import "terminal-client/types"

// 为了兼容性，导出 types 包中的函数
var (
	CloneTerminal    = types.CloneTerminal
	DefaultTerminal  = types.DefaultTerminal
	NormalizeTerminal = types.NormalizeTerminal
	ThemeColors      = types.ThemeColors
	AvailableThemes  = types.AvailableThemes
	HasTheme         = types.HasTheme
	AvailableFonts   = types.AvailableFonts
)
