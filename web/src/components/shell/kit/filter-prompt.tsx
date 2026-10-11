import type { Ref } from "react";

interface FilterPromptProps {
	label: string;
	onChange: (value: string) => void;
	placeholder: string;
	ref: Ref<HTMLInputElement>;
	value: string;
}

export function FilterPrompt({ label, onChange, placeholder, ref, value }: FilterPromptProps) {
	return (
		<div className="filter-prompt">
			<span aria-hidden className="text-primary">
				/
			</span>
			<input
				aria-label={label}
				className="min-w-0 flex-1 bg-transparent text-foreground outline-none placeholder:text-faint"
				onChange={(event) => onChange(event.target.value)}
				onKeyDown={(event) => {
					if (event.key !== "Escape") return;
					onChange("");
					event.currentTarget.blur();
				}}
				placeholder={placeholder}
				ref={ref}
				spellCheck={false}
				value={value}
			/>
		</div>
	);
}
