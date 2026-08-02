exports.handler = async () => {
  const now = new Date().toLocaleTimeString('ja-JP')
  const sources = ['fswatch', 'watchman', 'entr', 'logstream']
  const paths = ['/var/log/system.log', '/var/tmp/tmp1234', '/var/db/periodic', '/var/run/utmp']
  const events = ['Created', 'Modified', 'Deleted', 'Renamed']

  const logs = Array.from({ length: 3 }, () => ({
    ts: now,
    source: sources[Math.floor(Math.random() * sources.length)],
    message: `${paths[Math.floor(Math.random() * paths.length)]} → ${events[Math.floor(Math.random() * events.length)]}`,
  }))

  return {
    statusCode: 200,
    headers: { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' },
    body: JSON.stringify({ logs }),
  }
}
