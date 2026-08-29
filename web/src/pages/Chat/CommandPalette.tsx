import { useState, useRef, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Search, MessageSquarePlus, List, ArrowRightLeft, Eye, History,
  Square, Brain, Cpu, Languages, Layers, Activity, Stethoscope, Info,
  Settings, Timer, HeartPulse, Terminal, Tag, Upload, Trash2,
  FolderOpen, HelpCircle, User, BookOpen,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import type { AgentCapabilityManifest } from '@/api/projects';

export interface SlashCommand {
  cmd: string;
  labelKey: string;
  icon: React.ElementType;
  group: 'session' | 'settings' | 'info' | 'advanced';
  local?: boolean; // handled locally, not sent to bridge
  label?: string;
  labelZh?: string;
  availability?: 'available' | 'conditional' | 'unavailable';
  availabilityReason?: string;
  availabilityReasonZh?: string;
}

export const slashCommands: SlashCommand[] = [
  // Session
  { cmd: '/new', labelKey: 'cmd.new', icon: MessageSquarePlus, group: 'session' },
  { cmd: '/list', labelKey: 'cmd.list', icon: List, group: 'session' },
  { cmd: '/switch', labelKey: 'cmd.switch', icon: ArrowRightLeft, group: 'session' },
  { cmd: '/current', labelKey: 'cmd.current', icon: Eye, group: 'session' },
  { cmd: '/history', labelKey: 'cmd.history', icon: History, group: 'session' },
  { cmd: '/stop', labelKey: 'cmd.stop', icon: Square, group: 'session' },
  // Settings
  { cmd: '/model', labelKey: 'cmd.model', icon: Brain, group: 'settings' },
  { cmd: '/reasoning', labelKey: 'cmd.reasoning', icon: Cpu, group: 'settings' },
  { cmd: '/mode', labelKey: 'cmd.mode', icon: Layers, group: 'settings' },
  { cmd: '/lang', labelKey: 'cmd.lang', icon: Languages, group: 'settings' },
  { cmd: '/provider', labelKey: 'cmd.provider', icon: Activity, group: 'settings' },
  // Info
  { cmd: '/status', labelKey: 'cmd.status', icon: Info, group: 'info' },
  { cmd: '/help', labelKey: 'cmd.help', icon: HelpCircle, group: 'info' },
  { cmd: '/doctor', labelKey: 'cmd.doctor', icon: Stethoscope, group: 'info' },
  { cmd: '/version', labelKey: 'cmd.version', icon: Tag, group: 'info' },
  { cmd: '/whoami', labelKey: 'cmd.whoami', icon: User, group: 'info' },
  { cmd: '/commands', labelKey: 'cmd.commands', icon: Terminal, group: 'info' },
  // Advanced
  { cmd: '/dir', labelKey: 'cmd.dir', icon: FolderOpen, group: 'advanced' },
  { cmd: '/cron', labelKey: 'cmd.cron', icon: Timer, group: 'advanced' },
  { cmd: '/heartbeat', labelKey: 'cmd.heartbeat', icon: HeartPulse, group: 'advanced' },
  { cmd: '/alias', labelKey: 'cmd.alias', icon: Tag, group: 'advanced' },
  { cmd: '/config', labelKey: 'cmd.config', icon: Settings, group: 'advanced' },
  { cmd: '/skills', labelKey: 'cmd.skills', icon: BookOpen, group: 'advanced' },
  { cmd: '/upgrade', labelKey: 'cmd.upgrade', icon: Upload, group: 'advanced' },
  { cmd: '/delete-mode', labelKey: 'cmd.deleteMode', icon: Trash2, group: 'advanced' },
];

const fallbackByCommand = new Map(slashCommands.map((command) => [command.cmd, command]));

const manifestGroup = (category: string): SlashCommand['group'] => {
  if (category === 'session') return 'session';
  if (category === 'agent') return 'settings';
  if (category === 'system') return 'info';
  return 'advanced';
};

export function manifestToSlashCommands(manifest: AgentCapabilityManifest): SlashCommand[] {
  const commands: SlashCommand[] = manifest.commands.map((command) => {
    const token = command.invocation.split(/\s+/)[0];
    const fallback = fallbackByCommand.get(token);
    return {
      cmd: token,
      labelKey: fallback?.labelKey || 'cmd.commands',
      icon: fallback?.icon || Terminal,
      group: manifestGroup(command.category),
      label: command.description,
      labelZh: command.description_zh,
      availability: command.availability.state,
      availabilityReason: command.availability.reason,
      availabilityReasonZh: command.availability.reason_zh,
    };
  });
  const seen = new Set(commands.map((command) => command.cmd));
  for (const skill of manifest.skills) {
    const token = skill.invocation.split(/\s+/)[0];
    if (seen.has(token)) continue;
    seen.add(token);
    commands.push({
      cmd: token,
      labelKey: 'cmd.skills',
      icon: BookOpen,
      group: 'advanced',
      label: skill.display_name || skill.description || skill.name,
      labelZh: skill.display_name || skill.description || skill.name,
      availability: skill.availability.state,
      availabilityReason: skill.availability.reason,
      availabilityReasonZh: skill.availability.reason_zh,
    });
  }
  // Preserve Web-only navigation actions that are not ordinary chat commands.
  for (const fallback of slashCommands) {
    if (!seen.has(fallback.cmd) && fallback.cmd === '/delete-mode') {
      commands.push(fallback);
    }
  }
  return commands;
}

const groupOrder: { key: string; labelKey: string }[] = [
  { key: 'session', labelKey: 'cmd.groupSession' },
  { key: 'settings', labelKey: 'cmd.groupSettings' },
  { key: 'info', labelKey: 'cmd.groupInfo' },
  { key: 'advanced', labelKey: 'cmd.groupAdvanced' },
];

interface Props {
  open: boolean;
  onClose: () => void;
  onSelect: (cmd: SlashCommand) => void;
  anchorRef: React.RefObject<HTMLElement | null>;
  commands?: SlashCommand[];
}

export default function CommandPalette({ open, onClose, onSelect, anchorRef, commands }: Props) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState('');
  const [activeIdx, setActiveIdx] = useState(0);
  const panelRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    const availableCommands = commands || slashCommands;
    if (!query) return availableCommands;
    const q = query.toLowerCase().replace(/^\//, '');
    return availableCommands.filter(
      (c) => c.cmd.toLowerCase().includes(q) || (c.label || c.labelZh || t(c.labelKey)).toLowerCase().includes(q),
    );
  }, [commands, query, t]);

  useEffect(() => {
    if (open) {
      setQuery('');
      setActiveIdx(0);
      setTimeout(() => searchRef.current?.focus(), 50);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node) &&
          anchorRef.current && !anchorRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open, onClose, anchorRef]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIdx((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter' && filtered[activeIdx] && filtered[activeIdx].availability !== 'unavailable') {
      e.preventDefault();
      onSelect(filtered[activeIdx]);
    } else if (e.key === 'Escape') {
      onClose();
    }
  };

  if (!open) return null;

  const grouped = groupOrder
    .map((g) => ({
      ...g,
      items: filtered.filter((c) => c.group === g.key),
    }))
    .filter((g) => g.items.length > 0);

  let flatIdx = 0;

  return (
    <div
      ref={panelRef}
      className={cn(
        'absolute bottom-full left-0 mb-2 w-80 max-h-[420px] flex flex-col rounded-xl overflow-hidden z-50',
        'bg-white/95 backdrop-blur-xl border border-gray-200/80 shadow-2xl shadow-black/15',
        'dark:bg-[rgba(20,20,20,0.96)] dark:border-white/[0.1] dark:shadow-black/50',
        'animate-in slide-in-from-bottom-2 fade-in duration-200',
      )}
    >
      <div className="px-3 pt-3 pb-2">
        <div className="flex items-center gap-2 px-2.5 py-2 rounded-lg bg-gray-100/80 dark:bg-white/[0.06] border border-gray-200/50 dark:border-white/[0.05]">
          <Search size={14} className="text-gray-400 shrink-0" />
          <input
            ref={searchRef}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setActiveIdx(0); }}
            onKeyDown={handleKeyDown}
            placeholder={t('cmd.search', 'Search commands...')}
            className="flex-1 bg-transparent text-sm text-gray-900 dark:text-white placeholder:text-gray-400 outline-none"
          />
        </div>
      </div>
      <div className="flex-1 overflow-y-auto px-1.5 pb-2">
        {grouped.map((g) => (
          <div key={g.key} className="mb-1">
            <div className="px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
              {t(g.labelKey)}
            </div>
            {g.items.map((cmd) => {
              const idx = flatIdx++;
              const Icon = cmd.icon;
              const unavailable = cmd.availability === 'unavailable';
              const label = i18n.language.toLowerCase().startsWith('zh') ? (cmd.labelZh || cmd.label) : cmd.label;
              const reason = i18n.language.toLowerCase().startsWith('zh') ? (cmd.availabilityReasonZh || cmd.availabilityReason) : cmd.availabilityReason;
              return (
                <button
                  key={cmd.cmd}
                  type="button"
                  onClick={() => { if (!unavailable) onSelect(cmd); }}
                  onMouseEnter={() => setActiveIdx(idx)}
                  disabled={unavailable}
                  title={reason}
                  className={cn(
                    'w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-sm transition-colors',
                    unavailable && 'opacity-45 cursor-not-allowed',
                    idx === activeIdx
                      ? 'bg-accent/15 text-gray-900 dark:text-white'
                      : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100/80 dark:hover:bg-white/[0.06]',
                  )}
                >
                  <Icon size={15} className={idx === activeIdx ? 'text-accent' : 'text-gray-400'} />
                  <span className="font-mono text-xs text-gray-500 dark:text-gray-400 w-24 text-left">{cmd.cmd}</span>
                  <span className="flex-1 text-left truncate">{label || t(cmd.labelKey)}</span>
                </button>
              );
            })}
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="text-center text-sm text-gray-400 py-6">{t('common.noData')}</div>
        )}
      </div>
    </div>
  );
}
