<script lang="ts">
	interface Point { value: number }
	let { data = [] as Point[], width = 80, height = 30, 	color = "var(--chart-1)" }: {
		data: Point[];
		width?: number;
		height?: number;
		color?: string;
	} = $props();

	let d = $derived.by(() => {
		if (data.length < 2) return "";
		const values = data.map(d => d.value);
		const min = Math.min(...values);
		const max = Math.max(...values);
		const range = max - min || 1;
		const stepX = width / (data.length - 1);
		return values.map((v, i) => {
			const x = i * stepX;
			const y = height - ((v - min) / range) * height;
			return `${i === 0 ? "M" : "L"}${x},${y}`;
		}).join("");
	});
</script>

<svg {width} {height} class="overflow-visible">
	{#if data.length >= 2}
		<defs>
			<linearGradient id="sparkline-fill" x1="0" y1="0" x2="0" y2="1">
				<stop offset="0%" stop-color={color} stop-opacity="0.3" />
				<stop offset="100%" stop-color={color} stop-opacity="0.05" />
			</linearGradient>
		</defs>
		<path {d} fill="none" stroke={color} stroke-width="1.5" stroke-linecap="round" />
	{/if}
</svg>
