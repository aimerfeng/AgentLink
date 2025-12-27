import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export default function CreatorDashboard() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Creator Dashboard</h1>
      
      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatsCard title="Total Revenue" value="$0.00" change="+0%" />
        <StatsCard title="Total Calls" value="0" change="+0%" />
        <StatsCard title="Active Agents" value="0" change="+0" />
        <StatsCard title="Avg Rating" value="N/A" change="" />
      </div>

      {/* Recent Activity */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">
            No recent activity. Create your first Agent to get started!
          </p>
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
