import { type VariantProps, cva } from 'class-variance-authority'

export { default as Button } from './Button.vue'

export const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all duration-150 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground [box-shadow:var(--shadow-sm),inset_0_1px_0_rgba(255,255,255,0.12)] hover:brightness-105 hover:[box-shadow:var(--shadow-md),inset_0_1px_0_rgba(255,255,255,0.15)] active:scale-[0.97] active:[transition-duration:100ms]',
        destructive:
          'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90',
        outline:
          'border border-border-strong bg-surface text-ink hover:bg-surface-2 hover:border-ink',
        secondary:
          'bg-surface-2 text-ink border border-border hover:bg-muted',
        ghost:
          'text-muted-foreground hover:bg-surface-2 hover:text-ink',
        link:
          'text-primary underline-offset-4 hover:underline',
        accent:
          'bg-accent text-accent-foreground shadow-sm hover:bg-accent/90',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 px-3 text-xs',
        lg: 'h-10 px-6',
        xl: 'h-11 px-7 text-[15px]',
        icon: 'h-9 w-9',
        'icon-sm': 'h-8 w-8',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export type ButtonVariants = VariantProps<typeof buttonVariants>
