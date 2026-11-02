
export const formatJson = (input: unknown) => {
  if (input === null || input === undefined) {
    return { output: "", error: null };
  }

  if (typeof input !== "string") {
    try {
      return { output: JSON.stringify(input, null, "\t"), error: null };
    } catch {
      return { output: String(input), error: "Unable to stringify JSON value" };
    }
  }

  const text = input;
  if (!text.trim()) {
    return { output: "", error: null };
  }

  try {
    const parsed = JSON.parse(text);
    return { output: JSON.stringify(parsed, null, "\t"), error: null };
  } catch (e: unknown) {
    return {
      output: text,
      error: (e as { message: string }).message || "Invalid JSON",
    };
  }
};

