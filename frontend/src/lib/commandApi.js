function bindings() {
  if (typeof window === 'undefined') return null
  return window.go?.bindings?.CommandBindings || null
}

function requireBindings() {
  const value = bindings()
  if (!value) throw new Error('The quick command runtime is unavailable.')
  return value
}

export function hasCommandRuntime() {
  return bindings() !== null
}

export async function initialState() {
  return requireBindings().InitialState()
}

export async function connect(passphrase = '') {
  return requireBindings().Connect(passphrase)
}

export async function completePath(pathPrefix) {
  return requireBindings().CompletePath(pathPrefix)
}

export async function runCommand(command) {
  return requireBindings().RunCommand(command)
}

export async function cancelCommand() {
  return requireBindings().CancelCommand()
}

export async function deleteHistory(entryId) {
  return requireBindings().DeleteHistory(entryId)
}

export async function close() {
  return requireBindings().Close()
}
