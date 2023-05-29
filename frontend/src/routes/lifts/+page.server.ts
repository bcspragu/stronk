interface Weight {
	Unit: string;
	Value: number;
}

export interface Set {
	RepTarget: number;
	ToFailure: boolean;
	TrainingMaxPercentage: number;
	WeightTarget: Weight;
}

export interface Movement {
	Exercise: string;
	SetType: string;
	Sets: Set[];
}

interface NextLiftResponse {
	DayNumber: number;
	WeekNumber: number;
	IterationNumber: number;
	DayName: string;
	WeekName: string;
	Workout: Movement[];
	NextMovementIndex: number;
	NextSetIndex: number;
}

/** @type {import('./$types').PageLoad} */
export async function load({ fetch }) {
	const res = await fetch('http://localhost:8080/api/nextLift');
	const data: NextLiftResponse = await res.json();
	return data;
}
