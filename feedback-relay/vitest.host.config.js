import base from "./vitest.config.js";
import {defineConfig} from "vitest/config";

export default defineConfig({
  ...base,
  test: {
    ...base.test,
    include: ["test/host-auth.runtime.spec.js"],
  },
});
