{ Externe Routinen des Hosts (Modul "test", bereitgestellt vom Test-Harness) }
program Extern;

type
  TBinOp = function(A, B: LongInt): LongInt;
  TSingleOp = function(X: Single): Single;

function Add(A, B: LongInt): LongInt; external 'test' name 'add';
function Add64(A, B: Int64): Int64; external 'test' name 'add64';
function Halb(X: Real): Real; external 'test' name 'half';
function Twice(X: Single): Single; external 'test' name 'twice';
function Negate(B: Boolean): Boolean; external 'test' name 'not';
procedure Store(var X: LongInt; V: LongInt); external 'test' name 'store';
procedure Print(P: Pointer; Len: LongInt); external 'test' name 'print';
procedure PrintStr(const S: string); external 'test' name 'printstr';
function half(X: Real): Real; external 'test';   { Importname = Pascal-Name }
function Plus(A, B: LongInt): LongInt; external 'test' name 'add';  { derselbe Import }

var
  Op: TBinOp;
  SOp: TSingleOp;
  X: LongInt;
  S: string;

begin
  WriteLn(Add(2, 3), ' ', Plus(4, 5));
  WriteLn(Add64(5000000000, 1));
  WriteLn(Halb(5):0:2, ' ', half(1):0:2, ' ', Twice(1.25):0:2);
  WriteLn(Negate(True), ' ', Negate(False));
  Store(X, 42);
  WriteLn(X);
  S := 'Hallo vom Host';
  Print(@S[1], Length(S));
  PrintStr('als String');
  Op := Add;
  WriteLn(Op(10, 20), ' ', @Op = @Add);
  SOp := Twice;                 { Single an der Grenze: über eine Hülle }
  WriteLn(SOp(3.5):0:1)
end.
