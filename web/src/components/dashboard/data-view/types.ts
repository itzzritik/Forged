import type { ColumnDef, RowData, SortingState, VisibilityState } from "@tanstack/react-table";

export type DataViewAction<TData = unknown> = {
	id: string;
	label: string;
	onClick: (item: TData) => void;
	variant?: "default" | "destructive";
};

export type DataViewEmptyState = {
	title: string;
	description?: string;
	actionLabel?: string;
	onAction?: () => void;
};

export type DataViewResponsive = "base" | "sm" | "md" | "lg" | "xl";

declare module "@tanstack/react-table" {
	interface ColumnMeta<TData extends RowData, TValue> {
		align?: "start" | "center" | "end";
		cellClassName?: string;
		headerClassName?: string;
		responsive?: DataViewResponsive;
		toggleable?: boolean;
	}
}

export type DataViewColumn<TData> = ColumnDef<TData, unknown>;

export type DataViewSelectionToolbar<TData> = {
	label: (selectedRows: TData[]) => string;
	primaryActionLabel: (selectedRows: TData[]) => string;
	onPrimaryAction: (selectedRows: TData[]) => void;
};

export type DataViewProps<TData> = {
	data: TData[];
	columns: DataViewColumn<TData>[];
	entityLabel: string;
	globalFilterPlaceholder?: string;
	getRowId?: (item: TData, index: number) => string;
	getSearchText?: (item: TData) => string;
	isLoading?: boolean;
	emptyState?: DataViewEmptyState;
	initialSorting?: SortingState;
	initialColumnVisibility?: VisibilityState;
	rowHeight?: number;
	onRowClick?: (item: TData) => void;
	actions?: DataViewAction<TData>[] | ((item: TData) => DataViewAction<TData>[]);
	enableSelection?: boolean;
	selectionToolbar?: DataViewSelectionToolbar<TData>;
};
