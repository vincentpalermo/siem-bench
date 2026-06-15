declare module '../wailsjs/go/main/App' {
  export function GetStatus(): Promise<any>
  export function StartInfrastructure(): Promise<any>
  export function StopGoServices(): Promise<any>
  export function RunPreflight(): Promise<any>
  export function RunBenchmark(params: any): Promise<any>
  export function OpenResultsFolder(): Promise<void>
}

declare module '../wailsjs/runtime/runtime' {
  export function EventsOn(eventName: string, callback: (...args: any[]) => void): () => void
}
