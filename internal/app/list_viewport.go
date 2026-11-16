package app

import "fmt"

// listViewport is shared by rendering and hit testing. Window boundaries are
// recalculated from selection on resize, so focus can never scroll off screen.
type listViewport struct {
	start, end int
	columns    int
}

func listWindow(width, height, total, selected int, layout listLayout) listViewport {
	columns, visibleRows := 1, max(height-9, 1)
	if layout == cardsLayout {
		columns = cardLayoutColumns(width)
		visibleRows = max((height-5)/7, 1)
	}
	totalRows := (total + columns - 1) / columns
	selectedRow := clampIndex(selected, total) / columns
	startRow := min(max(selectedRow-visibleRows+1, 0), max(totalRows-visibleRows, 0))
	return listViewport{start: startRow * columns, end: min((startRow+visibleRows)*columns, total), columns: columns}
}

func (model *Model) listWindow(width, total int) listViewport {
	_, height := model.dimensions()
	selected := model.projectIndex
	if model.screen == boardsScreen {
		selected = model.boardIndex
	}
	return listWindow(width, height, total, selected, model.listLayout)
}

func (window listViewport) title(title string, total int) string {
	if window.start != 0 || window.end < total {
		return fmt.Sprintf("%s  %d–%d / %d", title, window.start+1, window.end, total)
	}
	return title
}
