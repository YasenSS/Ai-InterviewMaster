import { InterviewRoomPage } from "@/features/interviews/InterviewRoomPage";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <InterviewRoomPage id={id} />;
}
