import ConnectionsSidebar from './ConnectionsSidebar'
import SchemaExplorer from './SchemaExplorer'
import QueryEditor from './QueryEditor'

export default function DbManagerPage() {
  return (
    <div className="-m-6 flex h-[calc(100vh-3.5rem)] overflow-hidden">
      <div className="w-52 shrink-0">
        <ConnectionsSidebar />
      </div>
      <div className="w-64 shrink-0 border-r">
        <SchemaExplorer />
      </div>
      <div className="flex-1 overflow-hidden">
        <QueryEditor />
      </div>
    </div>
  )
}
