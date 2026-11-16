package app

type columnRect struct {
	index, x, width int
}

type workspaceLayout struct {
	width, height                       int
	navigationWidth, boardX, boardWidth int
	inspectorX, inspectorWidth          int
	columns                             []columnRect
}

func (model *Model) workspaceLayout(width, height int) workspaceLayout {
	layout := workspaceLayout{width: width, height: height, boardWidth: width}
	if model.screen != boardScreen {
		return layout
	}
	if model.workspace.inspector && width >= 110 {
		layout.inspectorWidth = min(38, width/3)
		layout.boardWidth -= layout.inspectorWidth
	}
	if model.workspace.navigation && width >= 140 && height >= 18 {
		layout.navigationWidth = 24
		layout.boardWidth -= layout.navigationWidth
		layout.boardX = layout.navigationWidth
	}
	layout.inspectorX = layout.boardX + layout.boardWidth
	if len(model.columns) == 0 {
		return layout
	}
	visible := min(len(model.columns), max(1, layout.boardWidth/22))
	start := min(max(0, model.columnIndex-visible+1), len(model.columns)-visible)
	x := layout.boardX
	for offset := 0; offset < visible; offset++ {
		columnWidth := layout.boardWidth / visible
		if offset < layout.boardWidth%visible {
			columnWidth++
		}
		layout.columns = append(layout.columns, columnRect{index: start + offset, x: x, width: columnWidth})
		x += columnWidth
	}
	return layout
}

func (layout workspaceLayout) columnAt(x int) (int, bool) {
	for _, column := range layout.columns {
		if x >= column.x && x < column.x+column.width {
			return column.index, true
		}
	}
	return 0, false
}

func (layout workspaceLayout) columnWidth(index int) int {
	for _, column := range layout.columns {
		if column.index == index {
			return column.width
		}
	}
	return max(layout.boardWidth/max(len(layout.columns), 1), 1)
}
