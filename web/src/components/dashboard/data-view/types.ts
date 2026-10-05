import type { ColumnDef, ColumnVisibilityState, RowData, SortingState } from "@tanstack/react-table";
import type { DataViewFeatures } from "./features";

export type { DataViewColumnMeta, DataViewResponsive } from "./features";

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

export type DataViewColumn<TData extends RowData> = ColumnDef<DataViewFeatures, TData, unknown>;

export type DataViewSelectionToolbar<TData> = {
	label: (selectedRows: TData[]) => string;
	primaryActionLabel: (selectedRows: TData[]) => string;
	onPrimaryAction: (selectedRows: TData[]) => void;
};

export type DataViewProps<TData extends RowData> = {
	data: TData[];
	columns: DataViewColumn<TData>[];
	entityLabel: string;
	globalFilterPlaceholder?: string;
	getRowId?: (item: TData, index: number) => string;
	getSearchText?: (item: TData) => string;
	isLoading?: boolean;
	emptyState?: DataViewEmptyState;
	initialSorting?: SortingState;
	initialColumnVisibility?: ColumnVisibilityState;
	rowHeight?: number;
	onRowClick?: (item: TData) => void;
	actions?: DataViewAction<TData>[] | ((item: TData) => DataViewAction<TData>[]);
	enableSelection?: boolean;
	selectionToolbar?: DataViewSelectionToolbar<TData>;
};
