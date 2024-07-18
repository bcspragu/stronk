import apipath from '$lib/apipath';
import type { PageLoad } from './$types';
import type { NextLiftResponse } from '$lib/api';

export const load: PageLoad = async ({ fetch }) => {
	const res = await fetch(apipath('/api/nextLift'));
	const data: NextLiftResponse = await res.json();
	return data;
};
