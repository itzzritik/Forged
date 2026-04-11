declare module "argon2-browser/dist/argon2-bundled.min.js" {
  interface Argon2HashResult {
    hash: ArrayBuffer;
    hashHex: string;
    encoded: string;
  }

  interface Argon2HashParams {
    pass: string | Uint8Array;
    salt: string | Uint8Array;
    time?: number;
    mem?: number;
    hashLen?: number;
    parallelism?: number;
    type?: number;
  }

  interface Argon2Module {
    hash(params: Argon2HashParams): Promise<Argon2HashResult>;
    ArgonType: {
      Argon2d: 0;
      Argon2i: 1;
      Argon2id: 2;
    };
  }

  const argon2: Argon2Module;
  export default argon2;
}
