import { JobDetailPage } from "@/features/jobs/JobDetailPage";
export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <JobDetailPage id={id} />;
}
