import type { ColumnDef, ColumnVisibilityState, RowData, SortingState } from "@tanstack/react-table";
import type { DataViewFeatures } from "./features";

export type { DataViewColumnMeta, DataViewResponsive } from "./features";

export interface DataViewAction<TData = unknown> {
	id: string;
	label: string;
	onClick: (item: TData) => void;
	variant?: "default" | "destructive";
}

export interface DataViewEmptyState {
	actionLabel?: string;
	description?: string;
	onAction?: () => void;
	title: string;
}

export type DataViewColumn<TData extends RowData> = ColumnDef<DataViewFeatures, TData, unknown>;

export interface DataViewSelectionToolbar<TData> {
	label: (selectedRows: TData[]) => string;
	onPrimaryAction: (selectedRows: TData[]) => void;
	primaryActionLabel: (selectedRows: TData[]) => string;
}

export interface DataViewProps<TData extends RowData> {
	actions?: DataViewAction<TData>[] | ((item: TData) => DataViewAction<TData>[]);
	columns: DataViewColumn<TData>[];
	data: TData[];
	emptyState?: DataViewEmptyState;
	enableSelection?: boolean;
	entityLabel: string;
	getRowId?: (item: TData, index: number) => string;
	getSearchText?: (item: TData) => string;
	globalFilterPlaceholder?: string;
	initialColumnVisibility?: ColumnVisibilityState;
	initialSorting?: SortingState;
	isLoading?: boolean;
	onRowClick?: (item: TData) => void;
	rowHeight?: number;
	selectionToolbar?: DataViewSelectionToolbar<TData>;
}
