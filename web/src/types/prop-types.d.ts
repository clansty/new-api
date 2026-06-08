declare module 'prop-types' {
  type Validator = {
    readonly isRequired: Validator;
  };

  type PropTypesShape = {
    readonly array: Validator;
    readonly bool: Validator;
    readonly func: Validator;
    readonly string: Validator;
    readonly oneOfType: (types: readonly Validator[]) => Validator;
  };

  const PropTypes: PropTypesShape;
  export default PropTypes;
}
