import { QuestionSetDetailPage } from "@/features/question-sets/QuestionSetDetailPage";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <QuestionSetDetailPage id={id} />;
}
