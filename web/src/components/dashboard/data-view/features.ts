import {
	columnOrderingFeature,
	columnPinningFeature,
	columnSizingFeature,
	columnVisibilityFeature,
	createSortedRowModel,
	metaHelper,
	rowSelectionFeature,
	rowSortingFeature,
	sortFn_alphanumeric,
	sortFn_basic,
	sortFn_datetime,
	sortFn_text,
	tableFeatures,
} from "@tanstack/react-table";

export type DataViewResponsive = "base" | "sm" | "md" | "lg" | "xl";

export interface DataViewColumnMeta {
	align?: "start" | "center" | "end";
	cellClassName?: string;
	headerClassName?: string;
	responsive?: DataViewResponsive;
	toggleable?: boolean;
}

// Sizing and ordering back the pinned-column offset and edge helpers; v8 bundled them implicitly.
// The sort registry holds the built-ins v8's "auto" sorting resolved to.
export const dataViewFeatures = tableFeatures({
	columnMeta: metaHelper<DataViewColumnMeta>(),
	columnOrderingFeature,
	columnPinningFeature,
	columnSizingFeature,
	columnVisibilityFeature,
	rowSelectionFeature,
	rowSortingFeature,
	sortedRowModel: createSortedRowModel(),
	sortFns: {
		alphanumeric: sortFn_alphanumeric,
		basic: sortFn_basic,
		datetime: sortFn_datetime,
		text: sortFn_text,
	},
});

export type DataViewFeatures = typeof dataViewFeatures;
