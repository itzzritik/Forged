import { Suspense } from "react";
import { KeysPage } from "@/components/shell/keys/keys-page";

export default function Page() {
	return (
		<Suspense>
			<KeysPage />
		</Suspense>
	);
}
