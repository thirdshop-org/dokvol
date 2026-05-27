import { derived, writable } from "svelte/store";
import translations from "./translations";

// translations/index.ts
export type TranslationLeaf = string;
export type TranslationNode = {
  [key: string]: TranslationNode | TranslationLeaf;
};
export type Translations = {
  [locale: string]: TranslationNode;
};


export const locale = writable("en");
export const locales = Object.keys(translations);


const typedTranslations = translations as Translations;

function getNestedValue(obj: TranslationNode, path: string[]): string | undefined {
  let current: TranslationNode | string = obj;

  for (const part of path) {
    if (typeof current === "object" && part in current) {
      current = current[part];
    } else {
      return undefined;
    }
  }

  return typeof current === "string" ? current : undefined;
}

function translate(
  locale: string,
  key: string,
  vars: Record<string, string | number> = {}
): string {
  if (!key) throw new Error("no key provided to $t()");

  const localeData = typedTranslations[locale];
  if (!localeData) throw new Error(`no translations found for locale '${locale}'`);

  let text = getNestedValue(localeData, key.split("."));

  // Fallback sur error.default pour les clés d'erreur inconnues
  if (!text && key.startsWith("error.")) {
    text = getNestedValue(localeData, ["error", "default"]);
  }

  if (!text) throw new Error(`no translation found for ${locale}.${key}`);
  for (const [k, v] of Object.entries(vars)) {
    text = text.replace(new RegExp(`{{${k}}}`, "g"), String(v));
  }

  return text;
}

export const t = derived(
  locale,
  ($locale) =>
    (key: string, vars: Record<string, string | number> = {}) =>
      translate($locale, key, vars)
);