unit Erste;
interface
const
  Name = 'erste';
  Wert = 1;
function Doppelt(x: Integer): Integer;
implementation
function Doppelt(x: Integer): Integer;
begin
  Doppelt := x + x
end;
end.
