import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export default function DeveloperDashboard() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Developer Console</h1>
      
      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatsCard title="API Calls" value="0" change="+0%" />
        <StatsCard title="Quota Remaining" value="100" change="" />
        <StatsCard title="Active Keys" value="0" change="" />
        <StatsCard title="Total Spent" value="$0.00" change="" />
      </div>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle>Quick Start</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            Get started by creating an API key and exploring the marketplace.
          </p>
          <div className="flex gap-4">
            <a
              href="/developer/keys"
              className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90"
            >
              Create API Key
            </a>
            <a
              href="/marketplace"
              className="rounded-md border px-4 py-2 text-sm hover:bg-muted"
            >
              Browse Agents
            </a>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function StatsCard({
  title,
  value,
  change,
}: {
  title: string;
  value: string;
  change: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {change && (
          <p className="text-xs text-muted-foreground">{change} from last month</p>
        )}
      </CardContent>
    </Card>
  );
}
