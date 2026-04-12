"use client";

import { Content, Overlay, Portal, Root } from "@radix-ui/react-dialog";
import { cn } from "@/lib/utils";

interface ModalProps {
	children: React.ReactNode;
	className?: string;
	closable?: boolean;
	onOpenChange: (open: boolean) => void;
	open: boolean;
	size?: "sm" | "md" | "lg" | "xl";
	title: string;
}

interface ModalSectionProps {
	children: React.ReactNode;
	className?: string;
}

const sizeClasses = {
	sm: "max-w-md",
	md: "max-w-lg",
	lg: "max-w-3xl",
	xl: "max-w-4xl",
} as const;

export const ModalBody = ({ children, className }: ModalSectionProps) => {
	return <div className={cn("flex flex-col gap-4 p-6", className)}>{children}</div>;
};

export const ModalFooter = ({ children, className }: ModalSectionProps) => {
	return <div className={cn("flex items-center justify-between gap-2 pt-1", className)}>{children}</div>;
};

export const Modal = ({ title, closable = true, open, onOpenChange, children, className, size = "md" }: ModalProps) => {
	return (
		<Root onOpenChange={closable ? onOpenChange : undefined} open={open}>
			<Portal>
				<Overlay className="data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-background/80 backdrop-blur-sm data-[state=closed]:animate-out data-[state=open]:animate-in" />
				<Content
					className={cn(
						"fixed top-1/2 left-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
						"w-full overflow-hidden border border-modal-shell-border bg-modal-shell font-mono shadow-2xl",
						sizeClasses[size],
						"data-[state=closed]:animate-out data-[state=open]:animate-in",
						"data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
						"data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95",
						className
					)}
					onEscapeKeyDown={closable ? undefined : (e) => e.preventDefault()}
					onInteractOutside={closable ? undefined : (e) => e.preventDefault()}
					onPointerDownOutside={closable ? undefined : (e) => e.preventDefault()}
				>
					<div className="flex h-[38px] items-center border-modal-titlebar-border border-b bg-modal-titlebar px-3.5">
						<div className="modal-window-controls flex items-center gap-2">
							<button
								aria-label="Close"
								className={cn(
									"modal-close-control group/modal-close relative flex h-[13px] w-[13px] items-center justify-center rounded-full border transition-[filter,box-shadow,background-color]",
									closable
										? "cursor-pointer border-modal-traffic-close/25 bg-modal-traffic-close hover:brightness-110 focus-visible:brightness-110 focus-visible:outline-none"
										: "cursor-default border-modal-traffic-close/20 bg-modal-traffic-close-soft"
								)}
								disabled={!closable}
								onClick={() => closable && onOpenChange(false)}
								style={closable ? { boxShadow: "0 0 14px var(--modal-traffic-close-glow)" } : undefined}
								type="button"
							>
								<svg
									aria-hidden
									className="pointer-events-none size-[8px] text-[var(--modal-traffic-close-icon)] opacity-0 transition-opacity group-hover/modal-close:opacity-100 group-focus-visible/modal-close:opacity-100"
									fill="none"
									stroke="currentColor"
									strokeLinecap="round"
									strokeLinejoin="round"
									strokeWidth="2.8"
									viewBox="0 0 10 10"
								>
									<path d="M2 2l6 6" />
									<path d="M8 2L2 8" />
								</svg>
							</button>
							<div
								aria-hidden
								className="modal-traffic-secondary-warning h-[13px] w-[13px] rounded-full border border-modal-traffic-warning/20 bg-modal-traffic-warning-soft transition-[background-color,border-color]"
							/>
							<div
								aria-hidden
								className="modal-traffic-secondary-success h-[13px] w-[13px] rounded-full border border-modal-traffic-success/20 bg-modal-traffic-success-soft transition-[background-color,border-color]"
							/>
						</div>
						<div className="flex-1 text-center text-[10px] text-muted-foreground uppercase tracking-[0.1em]">{title}</div>
						<div className="w-[58px]" />
					</div>
					{children}
				</Content>
			</Portal>
		</Root>
	);
};
