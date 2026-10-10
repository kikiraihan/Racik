export interface Toast {
  id: number
  text: string
  kind: 'info' | 'error' | 'success'
}

let seq = 0

export function useToast() {
  const toasts = useState<Toast[]>('toasts', () => [])
  function push(text: string, kind: Toast['kind'] = 'info', ms = 3500) {
    const id = ++seq
    toasts.value.push({ id, text, kind })
    setTimeout(() => (toasts.value = toasts.value.filter((t) => t.id !== id)), ms)
  }
  return {
    toasts,
    info: (t: string) => push(t, 'info'),
    success: (t: string) => push(t, 'success'),
    error: (t: string) => push(t, 'error', 6000),
  }
}
