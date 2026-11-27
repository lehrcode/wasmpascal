program ProzedurTypen;
type
  TProc = procedure;
  TFunc = function(X: Real): Real;
  TVar = procedure(var A: Integer; const S: string; B, C: LongInt);
  TRec = record
    F: function: Boolean;
    P: TProc
  end;
var
  P: TProc;
  Q: procedure(X: Integer);
  A: array[1..2] of TProc;
  PP: ^TProc;

procedure Hallo;
begin
end;

begin
  P := Hallo;
  P;
  Q(1);
  A[1] := Hallo;
  A[1];
  New(PP);
  PP^ := Hallo;
  PP^;
  if @P = nil then
    P := TProc(@Hallo)
end.
