"use client";

import * as DialogPrimitive from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

interface ModalProps {
	children: React.ReactNode;
	className?: string;
	closable?: boolean;
	onOpenChange: (open: boolean) => void;
	open: boolean;
	title: string;
}

export const Modal = ({ title, closable = true, open, onOpenChange, children, className }: ModalProps) => {
	return (
		<DialogPrimitive.Root onOpenChange={closable ? onOpenChange : undefined} open={open}>
			<DialogPrimitive.Portal>
				<DialogPrimitive.Overlay className="data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=closed]:animate-out data-[state=open]:animate-in" />
				<DialogPrimitive.Content
					className={cn(
						"fixed top-1/2 left-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
						"overflow-hidden border border-border bg-card shadow-2xl",
						"w-full max-w-md font-mono",
						"data-[state=closed]:animate-out data-[state=open]:animate-in",
						"data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
						"data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
						className
					)}
					onEscapeKeyDown={closable ? undefined : (e) => e.preventDefault()}
					onInteractOutside={closable ? undefined : (e) => e.preventDefault()}
					onPointerDownOutside={closable ? undefined : (e) => e.preventDefault()}
				>
					{/* Terminal title bar */}
					<div className="flex h-[38px] items-center border-border border-b bg-[#0a0a0a] px-3.5">
						<div className="flex items-center gap-[7px]">
							<button
								aria-label="Close"
								className={cn(
									"h-[11px] w-[11px] rounded-full border transition-colors",
									closable ? "cursor-pointer border-[#dc2626] bg-[#ef4444] hover:brightness-110" : "cursor-default border-[#3f1c20] bg-[#2a1215]"
								)}
								disabled={!closable}
								onClick={() => closable && onOpenChange(false)}
								type="button"
							/>
							<div aria-hidden className="h-[11px] w-[11px] rounded-full border border-[#3f3615] bg-[#2a2510]" />
							<div aria-hidden className="h-[11px] w-[11px] rounded-full border border-[#1a3f25] bg-[#0f2a18]" />
						</div>
						<div className="flex-1 text-center text-[10px] text-muted-foreground uppercase tracking-[0.1em]">{title}</div>
						<div className="w-[51px]" />
					</div>
					{children}
				</DialogPrimitive.Content>
			</DialogPrimitive.Portal>
		</DialogPrimitive.Root>
	);
};
