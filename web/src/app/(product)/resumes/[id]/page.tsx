import { ResumeDetailPage } from "@/features/resumes/ResumeDetailPage";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ResumeDetailPage id={id} />;
}
