import TasksPage from "./TasksPage";

/**
 * QuestionsPage — same UI as TasksPage but filtered to `board: 'questions'`,
 * which is where the Triager routes chats it classifies as casual / one-shot
 * Q&A rather than multi-step tasks.
 */
export default function QuestionsPage() {
  return (
    <TasksPage
      board="questions"
      title="Questions"
      emptyState="No quick questions yet — try chatting with your agent."
    />
  );
}
