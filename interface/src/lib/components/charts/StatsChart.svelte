<script lang="ts">
	import * as Card from "$lib/components/ui/card/index.js";
	import * as Chart from "$lib/components/ui/chart/index.js";
	import { scaleUtc } from "d3-scale";
	import { LineChart } from "layerchart";
	import { curveNatural } from "d3-shape";

	interface DataPoint {
		date: Date;
		value: number;
	}

	let {
		title = "",
		description = "",
		data = [] as DataPoint[],
		formatX = (v: Date) => v.toLocaleDateString(),
		formatY = (v: number) => v.toLocaleString(),
		color = "var(--chart-1)",
		height = "250px",
	}: {
		title?: string;
		description?: string;
		data?: DataPoint[];
		formatX?: (v: Date) => string;
		formatY?: (v: number) => string;
		color?: string;
		height?: string;
	} = $props();

	const chartConfig = $derived({
		value: { label: "Storage", color },
	} satisfies Chart.ChartConfig);

	const series = $derived([
		{ key: "value" as const, value: "value" as const, label: chartConfig.value.label, color: chartConfig.value.color },
	]);
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{title}</Card.Title>
		<Card.Description>{description}</Card.Description>
	</Card.Header>
	<Card.Content class="px-2 sm:p-6">
		<Chart.Container config={chartConfig} class="aspect-auto w-full" style="height: {height}">
			<LineChart
				data={data}
				x="date"
				y="value"
				xScale={scaleUtc()}
				series={series}
				props={{
					spline: { curve: curveNatural, motion: "tween", strokeWidth: 2 },
					xAxis: { format: formatX },
					yAxis: { format: formatY },
					highlight: { points: { r: 4 } },
				}}
			>
				{#snippet tooltip()}
					<Chart.Tooltip />
				{/snippet}
			</LineChart>
		</Chart.Container>
	</Card.Content>
</Card.Root>
