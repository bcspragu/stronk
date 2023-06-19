import apipath from '$lib/apipath';
import type { PageServerLoad } from './$types';
import type { NextLiftResponse } from '$lib/api';

export const load: PageServerLoad = async ({ fetch }) => {
	const res = await fetch(apipath('/api/nextLift'));
	const data: NextLiftResponse = await res.json();
	return data;
};
