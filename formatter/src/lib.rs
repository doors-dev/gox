use std::ptr;
mod formatter;
mod init;

/*
#[cfg(test)]
#[test]
fn test_simple() {
    let input = r#"
function weirdSum (a,b , c=1) {  // spacing + default
if(a==null||b==null){return 0}
const obj={x:1,y:{z:[3,2,1].map(n=>({n, sq:n*n}))}}
const msg=`sum=${a+b+c}`

for (const {n,sq} of obj.y.z) { if (sq%2===0) console.log(n , sq) }

try { JSON.parse("{bad}") } catch(e){ console.log("caught", e?.message) }

return {  a,b,c, obj , msg }
}

export  const  result=weirdSum(1,2)
console.log(result)
"#;
    let res = format_js_in_process(input).unwrap();
    println!("{res}");
} */


#[repr(C)]
pub struct Buf {
    pub ptr: *mut u8,
    pub len: usize,
    pub cap: usize,
    pub err: i32, // 0 ok, 1 input error, 2 format error
}



#[unsafe(no_mangle)]
pub extern "C" fn format(ptr_in: *const u8, len_in: usize) -> Buf {
    if ptr_in.is_null() && len_in != 0 {
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            err: 1,
        };
    }
    let bytes = unsafe { std::slice::from_raw_parts(ptr_in, len_in) };
    let mut output = Vec::new();
    let res = formatter::format(bytes, &mut output);
    if let Err(err) = res {
        let reason = match err {
            topiary_core::FormatterError::Idempotence => 2,
            topiary_core::FormatterError::IdempotenceParsing(_) => 3,
            topiary_core::FormatterError::Internal(_, _) => 4,
            topiary_core::FormatterError::Parsing(_) => 5,
            topiary_core::FormatterError::PatternDoesNotMatch => 6,
            topiary_core::FormatterError::Query(_, _) => 7,
            topiary_core::FormatterError::Io(_) => 8,
        };
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            err: reason,
        };
    }

    if output.is_empty() {
        return Buf {
            ptr: ptr::null_mut(),
            len: 0,
            cap: 0,
            err: 0,
        };
    }
    let len = output.len();
    let ptr = output.as_mut_ptr();
    let cap = output.capacity();
    std::mem::forget(output);
    Buf {
        ptr,
        len,
        cap,
        err: 0,
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn free_buf(ptr: *mut u8, len: usize, cap: usize) {
    if ptr.is_null() || cap == 0 {
        return;
    }
    unsafe {
        drop(Vec::from_raw_parts(ptr, len, cap));
    }
}
