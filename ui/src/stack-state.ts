import type {Container, Observed, Stack} from './types.js';

export function stackActions(stack: Pick<Stack, 'activeMode'>, observed: Observed | undefined, selectedMode: string, unavailable = false) {
  const known = Boolean(observed && observed.execution !== 'unknown');
  const available = known && !unavailable;
  const hasContainers = Boolean(observed?.containers.length);
  const hasRunning = Boolean(observed?.containers.some(c => c.running || ['restarting', 'paused'].includes(c.state)));
  const transitioning = Boolean(observed && (['starting', 'restarting'].includes(observed.execution) || observed.health === 'starting'));
  const sameModeRunning = observed?.execution === 'running' && stack.activeMode === selectedMode;
  const mode = selectedMode === 'verify' ? 'Prueba de imagen' : 'Desarrollo';
  return {
    start: available && !transitioning && !sameModeRunning,
    stop: available && hasRunning,
    restart: available && !transitioning && hasRunning,
    apply: available && !transitioning && hasContainers,
    build: available && !transitioning,
    startTitle: !known ? 'Actualiza el estado o conecta el runtime.' : transitioning ? 'Espera a que termine el inicio o reinicio.' : sameModeRunning ? `Ya está en ejecución en modo ${mode}. Usa Aplicar cambios para actualizar la configuración.` : stack.activeMode && stack.activeMode !== selectedMode ? `Iniciar cambiará al modo ${mode}; se pedirá confirmación.` : `Iniciar en modo ${mode}.`,
    stopTitle: !known ? 'Actualiza el estado o conecta el runtime.' : hasRunning ? 'Detener los servicios de la aplicación; se conservan sus datos.' : 'No hay servicios en ejecución.',
  };
}

export function healthDescription(value: string, containers: Pick<Container, 'service' | 'running' | 'hasHealthcheck'>[]) {
  const running = containers.filter(c => c.running);
  const missing = running.filter(c => c.hasHealthcheck === false);
  if (value !== 'unchecked' || !missing.length) return {};
  return {
    label: missing.length === running.length ? 'Sin comprobación de salud configurada' : 'Comprobación de salud incompleta',
    title: `Sin healthcheck en: ${missing.map(c => c.service).join(', ')}. Estar en ejecución no confirma que la aplicación esté lista.`,
  };
}
