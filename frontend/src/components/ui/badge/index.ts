import { type VariantProps, cva } from 'class-variance-authority'

export { default as Badge } from './Badge.vue'

export const badgeVariants = cva(
  'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold leading-tight transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-1',
  {
    variants: {
      variant: {
        default: 'bg-primary-soft text-primary border border-transparent',
        secondary: 'bg-muted text-muted-foreground border border-transparent',
        success: 'bg-success-soft text-success border border-transparent',
        warning: 'bg-warning-soft text-warning border border-transparent',
        info: 'bg-info-soft text-info border border-transparent',
        destructive: 'bg-danger-soft text-danger border border-transparent',
        accent: 'bg-accent-soft text-accent-strong border border-transparent',
        gold: 'bg-gold-soft text-gold border border-transparent',
        outline: 'border border-border-strong text-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export type BadgeVariants = VariantProps<typeof badgeVariants>
