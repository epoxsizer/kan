package app

type formViewport struct{ start, end, lineCount int }

func controlWindow(total, selected, visible int) (int, int) {
	start := min(max(selected-max(visible, 1)+1, 0), max(total-max(visible, 1), 0))
	return start, min(start+max(visible, 1), total)
}

func (model *Model) compactFormWindow(height int) formViewport {
	errorRows := 0
	if model.form.err != "" {
		errorRows = 2
	}
	visible := max(height-model.styles.help.GetVerticalFrameSize()-3-errorRows, 1)
	visible = min(visible, len(model.form.fields))
	start := min(max(model.form.focus-visible+1, 0), max(len(model.form.fields)-visible, 0))
	return formViewport{start: start, end: start + visible, lineCount: 3 + visible + errorRows}
}
