/*
* -----------------------------------------------------------
* In this package - CLI UI related components
* - color terminal
* - prompts
* - spinner / progress
* - ....
* -----------------------------------------------------------
 */

package ui

import (
	"github.com/fatih/color"
)

var (
	Notice  = color.New(color.Bold, color.FgGreen).PrintlnFunc()
	Success = color.New(color.Bold, color.FgGreen).PrintlnFunc()
	Info    = color.New(color.Bold, color.FgGreen).PrintlnFunc()
)
