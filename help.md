# how to use the bot

all image generation commands start with `!gen <prompt>`

in your prompt, you can use the following parameters:
| variable | values | example | explanation |
| --- | --- | --- | --- |
| `h`/`w` |  `64`-`768` | `h:320 w:512` | initial height/width. converted to multiples of 64. |
| `cfg` | `1`-`30` | `cfg:12` | cfg scale |
| `steps` | `1`-`150` | `steps:40` | sampling steps |
| `count` | `1`-`9` | `count:4` | number of images generated, will be returned in a grid |
| `hr` | `1`/`0` | `hr:1` | enable hr |
| `scale` | `1`-`4` | `scale:2.5` | upscaling factor when hr is enabled |
| `ds` | `0`-`1` | `ds:.6` | denoising strength |

## negative prompts

anything after `###` becomes a negative prompt. [read more](https://github.com/automatic1111/stable-diffusion-webui/wiki/features#negative-prompt)

## attention/emphasis, prompt editing, prompt composition

use `()` to make words have more weight, and `[]` to make them less important. [read more](https://github.com/automatic1111/stable-diffusion-webui/wiki/features#attentionemphasis)
use `[from:to:.2]` to change the prompt 20% through generation. [read more](https://github.com/automatic1111/stable-diffusion-webui/wiki/features#prompt-editing)
use photo of a `[skull|island|dog]` to alternate a prompt between different words on each step! [read more](https://github.com/AUTOMATIC1111/stable-diffusion-webui/wiki/Features#alternating-words)
use `AND` to separate prompts to have multiple positive prompts. [read more](https://github.com/automatic1111/stable-diffusion-webui/wiki/features#composable-diffusion)