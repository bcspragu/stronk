export interface Weight {
	Unit: string;
	Value: number;
}

export type SetType = 'WARMUP' | 'MAIN' | 'ASSISTANCE'
export type Exercise = 'OVERHEAD_PRESS' | 'SQUAT' | 'BENCH_PRESS' | 'DEADLIFT'

export interface Set {
	RepTarget: number;
	ToFailure: boolean;
	TrainingMaxPercentage: number;
	WeightTarget: Weight;
	FailureComparables?: ComparableLifts;
}

export interface Movement {
	Exercise: Exercise;
	SetType: SetType;
	Sets: Set[];
}

export interface Lift {
	Exercise:  Exercise;
	SetType: SetType;
	Weight: Weight;
	SetNumber: number;
	Reps: number;
	Note: string;

	DayNumber: number;
	WeekNumber: number;
	IterationNumber: number;
	ToFailure: boolean;
}

export interface ComparableLifts {
	ClosestWeight?: Lift
	PersonalRecord?: Lift
	PREquivalentReps: number
}

export interface NextLiftResponse {
	DayNumber: number;
	WeekNumber: number;
	IterationNumber: number;
	DayName: string;
	WeekName: string;
	Workout: Movement[];
	NextMovementIndex: number;
	NextSetIndex: number;
	OptionalWeek: boolean;
}

export interface TrainingMax {
	Max: Weight
	Exercise: Exercise
}

export interface TrainingMaxesResponse {
	TrainingMaxes: TrainingMax[]
	SmallestDenom?: Weight
}

export interface SetTrainingMaxesRequest {
		OverheadPress: string
		Squat: string
		BenchPress: string
		Deadlift: string
		SmallestDenom: string
}

export interface RecordLiftRequest {
  Exercise: Exercise
  SetType: SetType
  Weight: string
  Set: number
  Reps: number
  Note: string
  Day: number
  Week: number
  Iteration: number
	ToFailure: boolean
}

export interface SkipOptionalWeekRequest {
  Week: number
  Iteration: number
	Note: string
}
