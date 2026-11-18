// ui/lint: Vale alerts in the reader (markText) and the editor (backdrop),
// with a count button and panel (renderPanel) in the reader and the problems
// console (renderConsole) in the editor.
import "./lint.css";

export { clearMarks, flash, markText, severityOf, titleOf } from "./mark";
export { renderPanel, type LintPanel } from "./panel";
export { alertRange, backdrop, type Backdrop } from "./backdrop";
export { renderConsole, type LintConsole } from "./console";
