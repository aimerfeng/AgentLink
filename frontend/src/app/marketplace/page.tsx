import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

export default function MarketplacePage() {
  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="border-b bg-background">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4">
          <a href="/" className="text-xl font-bold text-primary">
            AgentLink
          </a>
          <nav className="flex items-center gap-4">
            <a href="/login" className="text-sm hover:text-primary">
              Sign In
            </a>
            <a
              href="/register"
              className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90"
            >
              Get Started
            </a>
          </nav>
        </div>
      </header>

      {/* Search Section */}
      <section className="border-b bg-muted/30 py-12">
        <div className="mx-auto max-w-7xl px-4">
          <h1 className="mb-6 text-center text-3xl font-bold">
            Discover AI Agents
          </h1>
          <div className="mx-auto flex max-w-2xl gap-2">
            <Input
              placeholder="Search agents by name, category, or keyword..."
              className="flex-1"
            />
            <Button>Search</Button>
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="py-8">
        <div className="mx-auto max-w-7xl px-4">
          <div className="flex flex-wrap gap-2">
            {['All', 'Writing', 'Coding', 'Analysis', 'Creative', 'Business'].map(
              (category) => (
                <Button
                  key={category}
                  variant={category === 'All' ? 'default' : 'outline'}
                  size="sm"
                >
                  {category}
                </Button>
              )
            )}
          </div>
        </div>
      </section>

      {/* Agent Grid */}
      <section className="py-8">
        <div className="mx-auto max-w-7xl px-4">
          <h2 className="mb-6 text-xl font-semibold">Featured Agents</h2>
          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {/* Placeholder cards */}
            <AgentCard
              name="Code Assistant"
              description="Expert coding assistant for multiple languages"
              price={0.01}
              rating={4.8}
              calls={1234}
            />
            <AgentCard
              name="Content Writer"
              description="Professional content writing and editing"
              price={0.02}
              rating={4.6}
              calls={856}
            />
            <AgentCard
              name="Data Analyst"
              description="Analyze data and generate insights"
              price={0.015}
              rating={4.7}
              calls={567}
            />
          </div>
        </div>
      </section>
    </div>
  );
}

function AgentCard({
  name,
  description,
  price,
  rating,
  calls,
}: {
  name: string;
  description: string;
  price: number;
  rating: number;
  calls: number;
}) {
  return (
    <Card className="transition-shadow hover:shadow-md">
      <CardHeader>
        <CardTitle className="text-lg">{name}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="mb-4 text-sm text-muted-foreground">{description}</p>
        <div className="flex items-center justify-between text-sm">
          <span className="font-medium">${price}/call</span>
          <span className="text-muted-foreground">
            ⭐ {rating} · {calls.toLocaleString()} calls
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
