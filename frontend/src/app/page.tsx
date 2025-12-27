import Link from 'next/link';
import { Button } from '@/components/ui/button';

export default function Home() {
  return (
    <main className="flex min-h-screen flex-col">
      {/* Hero Section */}
      <section className="flex flex-1 flex-col items-center justify-center px-4 py-24 text-center">
        <h1 className="mb-6 text-5xl font-bold tracking-tight sm:text-6xl lg:text-7xl">
          <span className="text-primary">Prompt</span> as Asset,{' '}
          <span className="text-primary">API</span> as Service
        </h1>
        <p className="mb-8 max-w-2xl text-lg text-muted-foreground sm:text-xl">
          AgentLink empowers AI creators to monetize their prompts securely while
          enabling developers to integrate AI capabilities with a single API call.
        </p>
        <div className="flex flex-col gap-4 sm:flex-row">
          <Link href="/register?type=creator">
            <Button size="lg" className="min-w-[200px]">
              Start Creating
            </Button>
          </Link>
          <Link href="/marketplace">
            <Button size="lg" variant="outline" className="min-w-[200px]">
              Explore Agents
            </Button>
          </Link>
        </div>
      </section>

      {/* Features Section */}
      <section className="bg-muted/50 px-4 py-24">
        <div className="mx-auto max-w-6xl">
          <h2 className="mb-12 text-center text-3xl font-bold">
            Why AgentLink?
          </h2>
          <div className="grid gap-8 md:grid-cols-3">
            <FeatureCard
              title="Secure Prompts"
              description="Your system prompts are encrypted and never exposed. Developers only see the API response."
              icon="🔒"
            />
            <FeatureCard
              title="Easy Integration"
              description="One API call to access any agent. No complex setup, just plug and play."
              icon="⚡"
            />
            <FeatureCard
              title="Transparent Revenue"
              description="Blockchain-verified settlements ensure fair and transparent revenue sharing."
              icon="💎"
            />
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="px-4 py-24 text-center">
        <h2 className="mb-6 text-3xl font-bold">Ready to Get Started?</h2>
        <p className="mb-8 text-muted-foreground">
          Join thousands of creators and developers on AgentLink
        </p>
        <Link href="/register">
          <Button size="lg">Create Free Account</Button>
        </Link>
      </section>
    </main>
  );
}

function FeatureCard({
  title,
  description,
  icon,
}: {
  title: string;
  description: string;
  icon: string;
}) {
  return (
    <div className="rounded-lg border bg-card p-6 shadow-sm">
      <div className="mb-4 text-4xl">{icon}</div>
      <h3 className="mb-2 text-xl font-semibold">{title}</h3>
      <p className="text-muted-foreground">{description}</p>
    </div>
  );
}
