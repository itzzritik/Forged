"use client";

import {
	type Column,
	type ColumnDef,
	type ColumnPinningState,
	type RowSelectionState,
	type SortingState,
	flexRender,
	getCoreRowModel,
	getSortedRowModel,
	useReactTable,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { CheckIcon, ChevronDownIcon, ChevronUpIcon, Columns3Icon, MoreHorizontalIcon, SearchIcon } from "lucide-react";
import { type CSSProperties, type MouseEvent, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from "@/components/ui/context-menu";
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { type DataViewAction, type DataViewColumn, type DataViewProps } from "./types";
import { useDataViewColumns } from "./use-data-view-columns";

const responsiveCN = {
	base: "",
	sm: "hidden sm:table-cell",
	md: "hidden md:table-cell",
	lg: "hidden lg:table-cell",
	xl: "hidden xl:table-cell",
} as const;

const alignCN = {
	start: "text-left",
	center: "text-center",
	end: "text-right",
} as const;

const PINNED_LEFT_SHADOW = "shadow-[inset_-1px_0_0_0_var(--data-view-border),8px_0_12px_-12px_rgba(0,0,0,0.35)]";
const PINNED_RIGHT_SHADOW = "shadow-[inset_1px_0_0_0_var(--data-view-border),-8px_0_12px_-12px_rgba(0,0,0,0.35)]";

function normalizeSearchValue(value: unknown): string {
	if (value == null) return "";
	if (typeof value === "string") return value.toLowerCase();
	if (typeof value === "number" || typeof value === "bigint" || typeof value === "boolean") return String(value).toLowerCase();
	if (value instanceof Date) return value.toISOString().toLowerCase();
	if (Array.isArray(value)) return value.map((item) => normalizeSearchValue(item)).join(" ");
	if (typeof value === "object") return Object.values(value as Record<string, unknown>).map((entry) => normalizeSearchValue(entry)).join(" ");
	return String(value).toLowerCase();
}

function SelectionCheckbox({
	ariaLabel,
	checked,
	indeterminate,
	onChange,
}: {
	ariaLabel: string;
	checked: boolean;
	indeterminate?: boolean;
	onChange: (checked: boolean) => void;
}) {
	const ref = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (ref.current) ref.current.indeterminate = Boolean(indeterminate) && !checked;
	}, [checked, indeterminate]);

	return (
		<input
			aria-label={ariaLabel}
			checked={checked}
			className="size-4 cursor-pointer rounded-none accent-primary"
			onChange={(event) => onChange(event.target.checked)}
			ref={ref}
			type="checkbox"
		/>
	);
}

function getPinnedCellStyle<TData>(column: Column<TData, unknown>, scope: "header" | "body"): CSSProperties | undefined {
	const pinned = column.getIsPinned();
	if (!pinned) return undefined;

	return {
		backgroundColor: scope === "header" ? "var(--data-view-sticky)" : "var(--data-view-shell)",
		backgroundClip: "padding-box",
		isolation: "isolate",
		left: pinned === "left" ? `${column.getStart("left")}px` : undefined,
		position: "sticky",
		right: pinned === "right" ? `${column.getAfter("right")}px` : undefined,
	};
}

function getPinnedCellClassName<TData>(column: Column<TData, unknown>, scope: "header" | "body") {
	const pinned = column.getIsPinned();
	if (!pinned) return null;

	return cn(
		"sticky",
		scope === "header" ? "z-20" : "z-10 bg-[var(--data-view-shell)]",
		pinned === "left" && column.getIsLastColumn("left") && PINNED_LEFT_SHADOW,
		pinned === "right" && column.getIsFirstColumn("right") && PINNED_RIGHT_SHADOW
	);
}

function isInteractiveTarget(target: HTMLElement) {
	return Boolean(target.closest("button,input,a,[role='menuitem'],[data-slot='dropdown-menu-trigger']"));
}

function DataViewActionMenuItems<TData>({
	actions,
	onAction,
}: {
	actions: DataViewAction<TData>[];
	onAction: (action: DataViewAction<TData>) => void;
}) {
	return (
		<>
			{actions.map((action) => (
				<DropdownMenuItem
					key={action.id}
					onClick={(event) => {
						event.stopPropagation();
						onAction(action);
					}}
					variant={action.variant === "destructive" ? "destructive" : "default"}
				>
					{action.label}
				</DropdownMenuItem>
			))}
		</>
	);
}

function DataViewRowActions<TData>({ actions, item }: { actions: DataViewAction<TData>[]; item: TData }) {
	if (actions.length === 0) return null;

	if (actions.length === 1) {
		const action = actions[0];
		if (!action) return null;
		return (
			<Button
				onClick={(event) => {
					event.stopPropagation();
					action.onClick(item);
				}}
				size="icon-sm"
				title={action.label}
				variant={action.variant === "destructive" ? "destructive" : "ghost"}
			>
				<span className="sr-only">{action.label}</span>
				<MoreHorizontalIcon className="size-4" />
			</Button>
		);
	}

	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				aria-label="Row actions"
				className="inline-flex size-7 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				onClick={(event) => event.stopPropagation()}
			>
				<MoreHorizontalIcon className="size-4" />
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end" className="w-40">
				<DataViewActionMenuItems actions={actions} onAction={(action) => action.onClick(item)} />
			</DropdownMenuContent>
		</DropdownMenu>
	);
}

function DataViewColumnToggle<TData>({
	disabled,
	table,
	onReset,
}: {
	disabled?: boolean;
	onReset?: () => void;
	table: ReturnType<typeof useReactTable<TData>>;
}) {
	const columns = table.getAllColumns().filter((column) => {
		if (column.id === "__select" || column.id === "__actions") return false;
		return true;
	});

	if (columns.length === 0) return null;

	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				aria-label="Toggle columns"
				className={cn(
					"inline-flex size-7 items-center justify-center rounded-lg border border-[var(--data-view-border)] bg-background text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
					disabled && "pointer-events-none opacity-50"
				)}
				disabled={disabled}
			>
				<Columns3Icon className="size-4" />
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end" className="w-52">
				<DropdownMenuGroup>
					<DropdownMenuLabel>Columns</DropdownMenuLabel>
					<DropdownMenuSeparator />
					{columns.map((column) => {
						const label = typeof column.columnDef.header === "string" ? column.columnDef.header : column.id;
						const locked = column.columnDef.meta?.toggleable === false;
						return (
							<DropdownMenuCheckboxItem
								checked={locked ? true : column.getIsVisible()}
								disabled={locked}
								key={column.id}
								onCheckedChange={locked ? undefined : (checked) => column.toggleVisibility(checked)}
							>
								{label}
							</DropdownMenuCheckboxItem>
						);
					})}
				</DropdownMenuGroup>
				{onReset && (
					<>
						<DropdownMenuSeparator />
						<DropdownMenuItem onClick={onReset}>Reset columns</DropdownMenuItem>
					</>
				)}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}

export function DataView<TData>({
	actions,
	columns,
	data,
	emptyState,
	enableSelection = false,
	entityLabel,
	getRowId,
	getSearchText,
	globalFilterPlaceholder,
	initialColumnVisibility,
	initialSorting,
	isLoading,
	onRowClick,
	rowHeight = 40,
	selectionToolbar,
}: DataViewProps<TData>) {
	const [sorting, setSorting] = useState<SortingState>(initialSorting ?? []);
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [searchText, setSearchText] = useState("");
	const deferredSearchText = useDeferredValue(searchText);
	const scrollRef = useRef<HTMLDivElement>(null);
	const resolveRowActions = (item: TData): DataViewAction<TData>[] => (typeof actions === "function" ? actions(item) : actions ?? []);

	const filteredData = useMemo(() => {
		const query = deferredSearchText.trim().toLowerCase();
		if (!query) return data;
		return data.filter((item) => normalizeSearchValue(getSearchText ? getSearchText(item) : item).includes(query));
	}, [data, deferredSearchText, getSearchText]);

	const initialVisibilityRecord = useMemo(() => {
		const visibility = { ...(initialColumnVisibility ?? {}) };
		if (enableSelection) visibility.__select = true;
		if (actions) visibility.__actions = true;
		return visibility;
	}, [actions, enableSelection, initialColumnVisibility]);

	const { cols: columnVisibility, setCols: setColumnVisibility, reset: resetColumnVisibility } = useDataViewColumns(entityLabel, initialVisibilityRecord);
	const columnPinning = useMemo<ColumnPinningState>(
		() => ({
			left: enableSelection ? ["__select"] : [],
			right: actions ? ["__actions"] : [],
		}),
		[actions, enableSelection]
	);

	const tableColumns = useMemo<ColumnDef<TData, unknown>[]>(() => {
		const nextColumns: ColumnDef<TData, unknown>[] = [];

		if (enableSelection) {
			nextColumns.push({
				id: "__select",
				enableHiding: false,
				enableSorting: false,
				header: ({ table }) => (
					<SelectionCheckbox
						ariaLabel={`Select all ${entityLabel}`}
						checked={table.getIsAllPageRowsSelected()}
						indeterminate={table.getIsSomePageRowsSelected()}
						onChange={(checked) => table.toggleAllPageRowsSelected(checked)}
					/>
				),
				cell: ({ row }) => (
					<SelectionCheckbox
						ariaLabel={`Select ${entityLabel}`}
						checked={row.getIsSelected()}
						onChange={(checked) => row.toggleSelected(checked)}
					/>
				),
				meta: {
					align: "center",
					cellClassName: "w-10",
					headerClassName: "w-10",
					toggleable: false,
				},
			});
		}

		nextColumns.push(...columns);

		if (actions) {
			nextColumns.push({
				id: "__actions",
				enableHiding: false,
				enableSorting: false,
				header: () => null,
				cell: ({ row }) => {
					return <DataViewRowActions actions={resolveRowActions(row.original)} item={row.original} />;
				},
				meta: {
					align: "end",
					cellClassName: "w-16",
					headerClassName: "w-16",
					toggleable: false,
				},
			});
		}

		return nextColumns;
	}, [actions, columns, enableSelection, entityLabel]);

	const table = useReactTable({
		columns: tableColumns,
		data: filteredData,
		enableColumnPinning: true,
		getCoreRowModel: getCoreRowModel(),
		getRowId,
		getSortedRowModel: getSortedRowModel(),
		onColumnVisibilityChange: (updater) => {
			const next = typeof updater === "function" ? updater(columnVisibility) : updater;
			setColumnVisibility(next);
		},
		onRowSelectionChange: setRowSelection,
		onSortingChange: setSorting,
		state: {
			columnPinning,
			columnVisibility,
			rowSelection,
			sorting,
		},
	});

	const rows = table.getRowModel().rows;
	const headerGroups = table.getHeaderGroups();
	const visibleColumns = table.getVisibleLeafColumns();
	const selectedRows = table.getSelectedRowModel().rows.map((row) => row.original);
	const totalCount = data.length;
	const filteredCount = rows.length;
	const firstDataColumnId = visibleColumns.find((column) => column.id !== "__select" && column.id !== "__actions")?.id;

	const rowVirtualizer = useVirtualizer({
		count: rows.length,
		estimateSize: () => rowHeight,
		getItemKey: (index) => rows[index]?.id ?? index,
		getScrollElement: () => scrollRef.current,
		overscan: 12,
	});

	const virtualRows = rowVirtualizer.getVirtualItems();
	const paddingTop = virtualRows.length > 0 ? virtualRows[0]?.start ?? 0 : 0;
	const paddingBottom =
		virtualRows.length > 0 ? rowVirtualizer.getTotalSize() - (virtualRows[virtualRows.length - 1]?.end ?? 0) : 0;

	return (
		<div className="overflow-hidden border border-[var(--data-view-border)] bg-[var(--data-view-shell)]">
				<div className="border-b border-[var(--data-view-border)] bg-[var(--data-view-sticky)]">
					{selectionToolbar && selectedRows.length > 0 && (
						<div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--data-view-border)] px-3 py-2">
							<p className="font-mono text-[11px] text-muted-foreground uppercase tracking-[0.12em]">{selectionToolbar.label(selectedRows)}</p>
							<Button onClick={() => selectionToolbar.onPrimaryAction(selectedRows)} size="sm" variant="destructive">
								{selectionToolbar.primaryActionLabel(selectedRows)}
							</Button>
						</div>
					)}
					<div className="flex flex-wrap items-center gap-3 px-3 py-2.5">
						<div className="relative min-w-0 flex-1">
							{isLoading ? (
								<Skeleton className="h-8 w-full border border-[var(--data-view-border)]" />
							) : (
								<>
									<SearchIcon className="-translate-y-1/2 pointer-events-none absolute top-1/2 left-2.5 size-4 text-muted-foreground/60" />
									<Input
										className="h-8 border-[var(--data-view-border)] bg-background pl-8 font-mono text-sm"
										disabled={isLoading}
										onChange={(event) => setSearchText(event.target.value)}
										placeholder={globalFilterPlaceholder ?? `Search ${entityLabel}`}
										value={searchText}
									/>
								</>
							)}
						</div>
						{isLoading ? <Skeleton className="h-7 w-7 border border-[var(--data-view-border)]" /> : <DataViewColumnToggle disabled={isLoading} onReset={resetColumnVisibility} table={table} />}
					</div>
				</div>

				{totalCount === 0 && !isLoading ? (
					<div className="flex min-h-48 flex-col items-center justify-center gap-2 px-6 py-12 text-center">
						<p className="font-medium text-sm">{emptyState?.title ?? `No ${entityLabel} yet`}</p>
						{emptyState?.description && <p className="max-w-md text-muted-foreground text-sm">{emptyState.description}</p>}
						{emptyState?.actionLabel && emptyState.onAction && (
							<Button onClick={emptyState.onAction} variant="outline">
								{emptyState.actionLabel}
							</Button>
						)}
					</div>
				) : (
					<>
						<div className="max-h-[min(65vh,42rem)] overflow-auto" ref={scrollRef}>
							<table className="w-full border-separate border-spacing-0 font-mono text-sm">
								<thead className="sticky top-0 z-10">
									{headerGroups.map((headerGroup) => (
										<tr key={headerGroup.id}>
											{headerGroup.headers.map((header) => {
												const align = header.column.columnDef.meta?.align ?? "start";
												const responsive = header.column.columnDef.meta?.responsive ?? "base";
												const sortDirection = header.column.getIsSorted();
												const sortable = header.column.getCanSort();
												return (
													<th
														className={cn(
															"border-b border-r border-[var(--data-view-border)] bg-[var(--data-view-sticky)] px-3 py-2 font-medium text-[11px] uppercase tracking-[0.12em] last:border-r-0",
															getPinnedCellClassName(header.column, "header"),
															alignCN[align],
															responsiveCN[responsive],
															header.column.columnDef.meta?.headerClassName
														)}
														key={header.id}
														style={getPinnedCellStyle(header.column, "header")}
													>
														{header.isPlaceholder ? null : isLoading ? (
															header.column.id === "__actions" ? null : (
																<Skeleton className={cn("h-3", header.column.id === "__select" ? "mx-auto w-4" : "w-16")} />
															)
														) : sortable ? (
															<button
																className={cn("flex w-full items-center gap-1.5", align === "end" ? "justify-end" : align === "center" ? "justify-center" : "justify-start")}
																onClick={header.column.getToggleSortingHandler()}
																type="button"
															>
																{flexRender(header.column.columnDef.header, header.getContext())}
																{sortDirection === "asc" ? (
																	<ChevronUpIcon className="size-3.5 text-primary" />
																) : sortDirection === "desc" ? (
																	<ChevronDownIcon className="size-3.5 text-primary" />
																) : null}
															</button>
														) : (
															flexRender(header.column.columnDef.header, header.getContext())
														)}
													</th>
												);
											})}
										</tr>
									))}
								</thead>
								<tbody>
									{isLoading ? (
										Array.from({ length: 8 }, (_, rowIndex) => (
											<tr className="border-b border-[var(--data-view-border)]" key={`loading-${rowIndex}`}>
												{visibleColumns.map((column) => {
													const align = column.columnDef.meta?.align ?? "start";
													const responsive = column.columnDef.meta?.responsive ?? "base";
													const isSelectionColumn = column.id === "__select";
													const isActionsColumn = column.id === "__actions";
													const isPrimaryColumn = column.id === firstDataColumnId;
													return (
														<td
															className={cn(
																"border-b border-r border-[var(--data-view-border)] px-3 py-2.5 align-middle last:border-r-0",
																getPinnedCellClassName(column, "body"),
																alignCN[align],
																responsiveCN[responsive],
																column.columnDef.meta?.cellClassName
															)}
															key={`${rowIndex}-${column.id}`}
															style={getPinnedCellStyle(column, "body")}
														>
															{isSelectionColumn ? (
																<Skeleton className="h-4 w-4" />
															) : isActionsColumn ? (
																<Skeleton className="ml-auto h-7 w-7" />
															) : isPrimaryColumn ? (
																<div className="space-y-2">
																	<Skeleton className="h-4 w-32" />
																	<Skeleton className="h-3 w-44" />
																</div>
															) : (
																<Skeleton className="h-4 w-full max-w-[14rem]" />
															)}
														</td>
													);
												})}
											</tr>
										))
									) : filteredCount === 0 ? (
										<tr>
											<td className="px-6 py-12 text-center" colSpan={table.getVisibleLeafColumns().length}>
												<p className="font-medium text-sm">No matching {entityLabel}</p>
												<p className="mt-1 text-muted-foreground text-sm">Try a different search query.</p>
											</td>
										</tr>
									) : (
										<>
											{paddingTop > 0 && (
												<tr>
													<td colSpan={table.getVisibleLeafColumns().length} style={{ height: paddingTop }} />
												</tr>
											)}
											{virtualRows.map((virtualRow) => {
												const row = rows[virtualRow.index];
												if (!row) return null;
												const visibleCells = row.getVisibleCells();
												const rowActions = resolveRowActions(row.original);
												const hasRowContextMenu = rowActions.length > 1;
												const rowCells = visibleCells.map((cell) => {
													const align = cell.column.columnDef.meta?.align ?? "start";
													const responsive = cell.column.columnDef.meta?.responsive ?? "base";
													return (
														<td
															className={cn(
																"border-b border-r border-[var(--data-view-border)] px-3 py-2.5 align-middle last:border-r-0",
																getPinnedCellClassName(cell.column, "body"),
																alignCN[align],
																responsiveCN[responsive],
																cell.column.columnDef.meta?.cellClassName
															)}
															key={cell.id}
															style={getPinnedCellStyle(cell.column, "body")}
														>
															{flexRender(cell.column.columnDef.cell, cell.getContext())}
														</td>
													);
												});

												const rowClassName = cn("group/row border-b border-[var(--data-view-border)] transition-colors hover:bg-muted/30", onRowClick && "cursor-pointer");
												const rowOnClick = onRowClick
													? (event: MouseEvent<HTMLElement>) => {
															const target = event.target as HTMLElement;
															if (isInteractiveTarget(target)) return;
															onRowClick(row.original);
														}
													: undefined;

												if (!hasRowContextMenu) {
													return (
														<tr className={rowClassName} key={row.id} onClick={rowOnClick}>
															{rowCells}
														</tr>
													);
												}

												return (
													<ContextMenu key={row.id}>
														<ContextMenuTrigger
															className={rowClassName}
															onClick={rowOnClick}
															onContextMenu={(event) => {
																const target = event.target as HTMLElement;
																if (isInteractiveTarget(target)) {
																	event.preventBaseUIHandler();
																}
															}}
															render={<tr />}
														>
															{rowCells}
														</ContextMenuTrigger>
														<ContextMenuContent className="w-40">
															{rowActions.map((action) => (
																<ContextMenuItem
																	key={action.id}
																	onClick={(event) => {
																		event.stopPropagation();
																		action.onClick(row.original);
																	}}
																	variant={action.variant === "destructive" ? "destructive" : "default"}
																>
																	{action.label}
																</ContextMenuItem>
															))}
														</ContextMenuContent>
													</ContextMenu>
												);
											})}
											{paddingBottom > 0 && (
												<tr>
													<td colSpan={table.getVisibleLeafColumns().length} style={{ height: paddingBottom }} />
												</tr>
											)}
										</>
									)}
								</tbody>
							</table>
						</div>

						<div className="flex h-8 items-center justify-between border-t border-[var(--data-view-border)] bg-[var(--data-view-sticky)] px-3 text-xs text-muted-foreground">
							{isLoading ? (
								<>
									<Skeleton className="h-3 w-20" />
									<Skeleton className="h-5 w-16" />
								</>
							) : (
								<>
									<span>
										{filteredCount !== totalCount ? (
									<>
										<span className="text-foreground">{filteredCount}</span> of {totalCount} {entityLabel}
									</>
								) : (
									<>
										{totalCount} {entityLabel}
									</>
								)}
									</span>
									{selectedRows.length > 0 && <Badge variant="outline">{selectedRows.length} selected</Badge>}
								</>
							)}
						</div>
					</>
				)}
		</div>
	);
}

export * from "./types";
export { useDataViewColumns } from "./use-data-view-columns";
