<script lang="ts" module>
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";
	import { type VariantProps, tv } from "tailwind-variants";

	export const badgeVariants = tv({
		base: "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 [&_svg]:pointer-events-none [&_svg]:size-3 [&_svg]:shrink-0",
		variants: {
			variant: {
				default: "border-transparent bg-primary text-primary-foreground shadow-sm hover:bg-primary/80",
				secondary: "border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80",
				destructive: "border-transparent bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/80",
				outline: "text-foreground",
				success: "border-transparent bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100",
				running: "border-transparent bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100",
				warning: "border-transparent bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-100",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type BadgeVariant = VariantProps<typeof badgeVariants>["variant"];

	export type BadgeProps = WithElementRef<HTMLAttributes<HTMLElement>> & {
		variant?: BadgeVariant;
	};
</script>

<script lang="ts">
	let {
		class: className,
		variant = "default",
		ref = $bindable(null),
		children,
		...restProps
	}: BadgeProps = $props();
</script>

<span
	bind:this={ref}
	data-slot="badge"
	class={cn(badgeVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</span>
