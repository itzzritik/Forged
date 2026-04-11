declare module "argon2-browser/dist/argon2-bundled.min.js" {
	interface Argon2HashResult {
		encoded: string;
		hash: ArrayBuffer;
		hashHex: string;
	}

	interface Argon2HashParams {
		hashLen?: number;
		mem?: number;
		parallelism?: number;
		pass: string | Uint8Array;
		salt: string | Uint8Array;
		time?: number;
		type?: number;
	}

	interface Argon2Module {
		ArgonType: {
			Argon2d: 0;
			Argon2i: 1;
			Argon2id: 2;
		};
		hash(params: Argon2HashParams): Promise<Argon2HashResult>;
	}

	const argon2: Argon2Module;
	export default argon2;
}
