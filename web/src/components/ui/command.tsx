"use client";

import { Command as CommandPrimitive } from "cmdk";
import { CheckIcon, SearchIcon } from "lucide-react";
import type * as React from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

function Command({ className, ...props }: React.ComponentProps<typeof CommandPrimitive>) {
	return (
		<CommandPrimitive
			className={cn("flex size-full flex-col overflow-hidden rounded-none bg-command-shell text-foreground", className)}
			data-slot="command"
			{...props}
		/>
	);
}

function CommandDialog({
	title = "Command Palette",
	description = "Search for a command to run...",
	children,
	className,
	showCloseButton = false,
	...props
}: Omit<React.ComponentProps<typeof Dialog>, "children"> & {
	title?: string;
	description?: string;
	className?: string;
	showCloseButton?: boolean;
	children: React.ReactNode;
}) {
	return (
		<Dialog {...props}>
			<DialogHeader className="sr-only">
				<DialogTitle>{title}</DialogTitle>
				<DialogDescription>{description}</DialogDescription>
			</DialogHeader>
				<DialogContent
					className={cn(
						"top-[18%] translate-y-0 overflow-hidden rounded-none border border-command-shell-border bg-command-shell p-0 font-mono shadow-2xl ring-0 sm:max-w-xl",
						className
					)}
				showCloseButton={showCloseButton}
			>
				{children}
			</DialogContent>
		</Dialog>
	);
}

function CommandInput({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Input>) {
	return (
		<div className="border-command-shell-border border-b bg-command-input px-3 py-2" data-slot="command-input-wrapper">
			<div className="flex items-center gap-2" data-slot="command-input-inner">
				<SearchIcon className="size-4 shrink-0 text-muted-foreground" />
				<CommandPrimitive.Input
					className={cn(
						"h-8 w-full border-0 bg-transparent text-sm text-foreground outline-hidden placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50",
						className
					)}
					data-slot="command-input"
					{...props}
				/>
			</div>
		</div>
	);
}

function CommandList({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.List>) {
	return (
		<CommandPrimitive.List
			className={cn("no-scrollbar max-h-[24rem] scroll-py-2 overflow-y-auto overflow-x-hidden px-2 py-2 outline-none", className)}
			data-slot="command-list"
			{...props}
		/>
	);
}

function CommandEmpty({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Empty>) {
	return <CommandPrimitive.Empty className={cn("py-6 text-center text-sm", className)} data-slot="command-empty" {...props} />;
}

function CommandGroup({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Group>) {
	return (
		<CommandPrimitive.Group
			className={cn(
				"overflow-hidden py-1 text-foreground **:[[cmdk-group-heading]]:px-2 **:[[cmdk-group-heading]]:pb-1 **:[[cmdk-group-heading]]:font-mono **:[[cmdk-group-heading]]:text-[10px] **:[[cmdk-group-heading]]:text-muted-foreground **:[[cmdk-group-heading]]:uppercase **:[[cmdk-group-heading]]:tracking-[0.16em]",
				className
			)}
			data-slot="command-group"
			{...props}
		/>
	);
}

function CommandSeparator({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Separator>) {
	return <CommandPrimitive.Separator className={cn("my-1 h-px bg-command-shell-border", className)} data-slot="command-separator" {...props} />;
}

function CommandItem({ className, children, ...props }: React.ComponentProps<typeof CommandPrimitive.Item>) {
	return (
		<CommandPrimitive.Item
			className={cn(
				"group/command-item relative flex cursor-default select-none items-center gap-2 border border-transparent px-2.5 py-2 text-sm outline-hidden transition-colors data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 data-selected:border-primary/25 data-selected:bg-command-item-selected data-selected:text-foreground [&_svg:not([class*='size-'])]:size-4 [&_svg]:pointer-events-none [&_svg]:shrink-0 data-selected:*:[svg]:text-foreground",
				className
			)}
			data-slot="command-item"
			{...props}
		>
			{children}
			<CheckIcon className="ml-auto opacity-0 group-has-data-[slot=command-shortcut]/command-item:hidden group-data-[checked=true]/command-item:opacity-100" />
		</CommandPrimitive.Item>
	);
}

function CommandShortcut({ className, ...props }: React.ComponentProps<"span">) {
	return (
		<span
			className={cn("ml-auto font-mono text-[10px] text-command-shortcut uppercase tracking-[0.16em] group-data-selected/command-item:text-foreground", className)}
			data-slot="command-shortcut"
			{...props}
		/>
	);
}

export { Command, CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator, CommandShortcut };
