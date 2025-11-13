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

export type Difficulty = {
  name: "Easy" | "Medium" | "Hard";
  value: "Easy" | "Medium" | "Hard";
};
export type QuestionStatus = Question["status"];
export type Category =
  {
    name: "Arrays and Strings",
    value: "Arrays_and_Strings"
  }
  | {
    name: "Linked Structures",
    value: "Linked_Structures"
  } |
  {
    name: "Hashing and Sets",
    value: "Hashing_and_Sets"
  } |
  {
    name: "Sorting and Selection",
    value: "Sorting_and_Selection"
  } |
  {
    name: "Graphs",
    value: "Graphs"
  } |
  {
    name: "Trees and Tries",
    value: "Trees_and_Tries"
  } |
  {
    name: "Heaps and Priority Structures",
    value: "Heaps_and_Priority_Structures"
  } |
  {
    name: "Algorithm Design Paradigms",
    value: "Algorithm_Design_Paradigms"
  } |
  {
    name: "Math and Number Theory",
    value: "Math_and_Number_Theory"
  } |
  {
    name: "System Design",
    value: "System_Design"
  };

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
