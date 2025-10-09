export interface Question {
  id: number;
  title: string;
  topic: string;
  difficulty: "Easy" | "Medium" | "Hard";
  status: "Solved" | "Attempted" | "Unsolved";
}

export type Difficulty = Question["difficulty"];
export type Status = Question["status"];