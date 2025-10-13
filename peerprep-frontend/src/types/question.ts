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

export interface RandomQuestionFilters {
  difficulty?: Difficulty;
  topic_tags?: string[];
}