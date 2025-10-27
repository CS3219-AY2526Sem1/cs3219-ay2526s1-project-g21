export interface TestCase {
  input: string;
  output: string;
  description?: string;
}

export interface Question {
  id: number;
  title: string;
  difficulty: "Easy" | "Medium" | "Hard";
  topic_tags: string[];
  prompt_markdown: string;
  constraints?: string;
  test_cases?: TestCase[];
  image_urls?: string[];
  status: "active" | "deprecated";
  author?: string;
  created_at: string;
  updated_at: string;
  deprecated_at?: string;
  deprecated_reason?: string;
}

export type Difficulty = Question["difficulty"];
export type QuestionStatus = Question["status"];
export type Category = "Array" | "Graphs" | "Dynamic Programming" | "Greedy" | "Linked List";

export interface RandomQuestionFilters {
  difficulty?: Difficulty;
  topic_tags?: string[];
}

export interface RoomInfo {
  matchId: string;
  user1: string;
  user2: string;
  category: string;
  difficulty: string;
  status: "pending" | "processing" | "ready" | "error";
  question?: Question;
  rerollsRemaining?: number;
  createdAt: string;
  token1?: string;
  token2?: string;
  token: string;
  type: "match_confirmed" | "match_pending" | "timeout" | "requeued"
}

type MatchConfirmEvent = {
  type: "match_confirmed",
  matchId: string,
  token: string,
  category: string,
  difficulty: string
}

type MatchPendingEvent = {
  type: "match_pending",
  matchId: string,
}

type MatchRequeueEvent = {
  type: "requeued"
}

type MatchTimeoutEvent = {
  type: "timeout"
}

export type MatchEvent = {
  type: "match_confirmed",
  matchId: string,
  token: string,
  category: string,
  difficulty: string
} | {
  type: "match_pending",
  matchId: string,
} | {
  type: "requeued"
} | {
  type: "timeout"
} 
