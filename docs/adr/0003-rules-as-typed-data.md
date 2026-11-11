# Rules are fixed-type data, not a DSL; the engine returns reason codes

Each rule type in the spec (F05.2) is a struct with fixed fields: recommended milestone with its anchor milestone (birth date / first dose / previous dose), minimum and maximum age, interval, buffer, flags… The logic lives entirely in engine code. Admin and AI only edit *parameters*, so the diff on the review page is easy to read, and the AI review checks numbers rather than checking a program. When a new rule type is needed, we change engine code and deploy.

The engine does not generate prose. Each assessment returns a bucket or block, the dates, and a list of **reason codes with parameters** (`SAFEST_SERIES`, `NOT_ENTERED`…). The web and the email templates translate the codes into Vietnamese. This way, test cases compare against stable codes, and adding other languages later does not require touching the engine.

## Considered Options

- A DSL or expression language (CEL, JSONLogic): more flexible, but each rule then becomes a program that must be verified, which is hard for both the AI review and the admin.
